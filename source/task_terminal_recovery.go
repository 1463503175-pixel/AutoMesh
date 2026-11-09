package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// terminalResultForJob reads the immutable, job-specific receipt first.
// A generic last_result.json may belong to an earlier quality check and may
// not be used as proof that this specific job has ended.
func (w Workspace) terminalResultForJob(jobID string) (Result, bool) {
	if len(jobID) != 32 {
		return Result{}, false
	}
	for _, path := range []string{w.path("job_" + jobID + ".json"), w.path("last_result.json")} {
		r, err := readJSON[Result](path)
		if err == nil && r.JobID == jobID && taskTerminal(r.Status) {
			return r, true
		}
	}
	return Result{}, false
}

// recoverWarningPreviewArtifact accepts success only when the worker wrote a
// plan that carries this exact job ID and matches the saved project snapshot.
// This is not inferred from a PID or a generic progress message.
func (w Workspace) recoverWarningPreviewArtifact(lease Task) (Result, bool, error) {
	if lease.Phase != "warning_refine_preview" || lease.JobID == "" {
		return Result{}, false, nil
	}
	b, err := os.ReadFile(w.path("warning_refinement_plan.json"))
	if err != nil {
		return Result{}, false, nil
	}
	var plan WarningPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return Result{}, false, nil
	}
	if plan.JobID != lease.JobID || plan.Project != lease.ExpectedProject || plan.Fingerprint != lease.Fingerprint || plan.PlanID == "" {
		return Result{}, false, nil
	}
	snapshot, err := w.LoadSnapshot()
	if err != nil || snapshot.Project != lease.ExpectedProject || snapshot.Fingerprint != lease.Fingerprint {
		return Result{}, false, nil
	}
	if err := validateWarningDiagnostic(plan, &snapshot, plan.PlanID); err != nil {
		return Result{}, false, err
	}
	summary, err := json.Marshal(plan)
	if err != nil {
		return Result{}, false, err
	}
	lease = normalizeTaskIdentity(lease)
	result := Result{JobID: lease.JobID, LogicalJobID: lease.LogicalJobID,
		AttemptID: lease.AttemptID, AttemptNumber: lease.AttemptNumber, RetryOfJobID: lease.RetryOfJobID,
		VerificationStatus: "verified", Phase: lease.Phase, Status: "ok", Summary: summary}
	if err := w.RecordWarningPreview(result, &snapshot); err != nil {
		return Result{}, false, err
	}
	result.Stage = "recovered_from_job_bound_warning_plan"
	return result, true, nil
}

// recoverTerminalProgress only accepts the terminal marker emitted by the
// ACT worker AFTER it has returned from its operation, in the exception or
// cooperative-cancellation handler. Neither a stale heartbeat nor an unreadable
// PID is evidence of completion. The job ID, phase, timestamp and exact marker
// all have to match. We deliberately do NOT reconstruct successful results:
// success without a report cannot establish what was actually created.
func (w Workspace) recoverTerminalProgress(lease Task) (Result, bool, error) {
	if len(lease.JobID) != 32 || lease.Phase == "" {
		return Result{}, false, nil
	}
	if r, ok := w.terminalResultForJob(lease.JobID); ok {
		return r, true, nil
	}
	lease = normalizeTaskIdentity(lease)
	progress, err := readJSON[ActProgress](w.path("job_progress_" + lease.JobID + ".json"))
	if err != nil || progress.JobID != lease.JobID || progress.Timestamp <= 0 ||
		(progress.AttemptID != "" && progress.AttemptID != lease.AttemptID) ||
		(progress.LogicalJobID != "" && progress.LogicalJobID != lease.LogicalJobID) {
		return Result{}, false, nil
	}
	age := time.Since(time.Unix(0, int64(progress.Timestamp*1e9)))
	if age < 10*time.Second || age > 7*24*time.Hour {
		return Result{}, false, nil
	}
	status := ""
	detail := strings.TrimSpace(progress.Detail)
	switch progress.Stage {
	case "任务失败":
		status = "error"
	case "已安全取消":
		status = "cancelled"
	case "终态结果全部写入失败", "部分回执写入失败":
		// The worker emits these stages only AFTER its requested ACT operation
		// has returned and terminal-receipt writes have been attempted. A preview
		// may be recovered as OK only if its own plan artifact proves the exact job.
		status = "error"
		if detail == "" {
			detail = "ACT 已进入终态回执写入阶段，但任务结果 JSON 不可用；操作是否成功仍需核查工程状态和方案文件。"
		} else {
			detail = "ACT 已进入终态回执写入阶段；不能据此推断操作成功或失败。回执写入诊断：" + detail
		}
	default:
		return Result{}, false, nil
	}
	var recoveredSummary json.RawMessage
	if (progress.Stage == "终态结果全部写入失败" || progress.Stage == "部分回执写入失败") && lease.Phase == "warning_refine_preview" {
		if recovered, ok, recoveryErr := w.recoverWarningPreviewArtifact(lease); recoveryErr == nil && ok {
			status = "ok"
			detail = ""
			recoveredSummary = recovered.Summary
		}
	}
	if detail == "" && status != "ok" {
		detail = "Meshing ACT signaled a terminal failure/cancellation without its final JSON receipt"
	}
	recoveredStage := "recovered_from_ACT_terminal_marker"
	if status == "ok" && len(recoveredSummary) > 0 {
		recoveredStage = "recovered_from_job_bound_warning_plan"
	}
	lease = normalizeTaskIdentity(lease)
	verificationStatus := "pending"
	if status == "ok" && recoveredStage == "recovered_from_job_bound_warning_plan" {
		verificationStatus = "verified"
	}
	r := Result{JobID: lease.JobID, LogicalJobID: lease.LogicalJobID,
		AttemptID: lease.AttemptID, AttemptNumber: lease.AttemptNumber, RetryOfJobID: lease.RetryOfJobID,
		VerificationStatus: verificationStatus, Phase: lease.Phase, Status: status,
		Stage: recoveredStage, Summary: recoveredSummary, Error: detail}
	// Only repair the AutoMesh workspace. This does not send commands to
	// Workbench, change controls, clear a mesh, or inspect/terminate a process.
	// Preserve the old last result before replacing it, when needed.
	if prior, e := readJSON[Result](w.path("last_result.json")); e == nil && prior.JobID != "" && prior.JobID != lease.JobID {
		oldPath := w.path("job_" + prior.JobID + ".json")
		if _, statErr := os.Stat(oldPath); errors.Is(statErr, os.ErrNotExist) {
			if e := writeJSON(oldPath, prior); e != nil {
				return Result{}, false, fmt.Errorf("cannot preserve previous result: %w", e)
			}
		}
	}
	// Per-job receipt is durable even if the generic result cannot be written.
	if e := writeJSON(w.path("job_"+lease.JobID+".json"), r); e != nil {
		return Result{}, false, fmt.Errorf("cannot repair terminal ACT receipt: %w", e)
	}
	// The immutable per-job receipt is authoritative. A generic last_result
	// replacement failure must not keep the completed task locked forever.
	_ = writeJSON(w.path("last_result.json"), r)
	if err := w.recordTaskResult(lease, r); err != nil {
		return Result{}, false, fmt.Errorf("terminal receipt recovered but task history could not be updated: %w", err)
	}
	return r, true, nil
}
