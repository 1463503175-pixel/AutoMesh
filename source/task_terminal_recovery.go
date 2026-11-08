package main

import (
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
	progress, err := readJSON[ActProgress](w.path("job_progress_" + lease.JobID + ".json"))
	if err != nil || progress.JobID != lease.JobID || progress.Timestamp <= 0 {
		return Result{}, false, nil
	}
	age := time.Since(time.Unix(0, int64(progress.Timestamp*1e9)))
	if age < 10*time.Second || age > 7*24*time.Hour {
		return Result{}, false, nil
	}
	status := ""
	switch progress.Stage {
	case "任务失败":
		status = "error"
	case "已安全取消":
		status = "cancelled"
	default:
		return Result{}, false, nil
	}
	detail := strings.TrimSpace(progress.Detail)
	if detail == "" {
		detail = "Meshing ACT signaled a terminal failure/cancellation without its final JSON receipt"
	}
	r := Result{JobID: lease.JobID, Phase: lease.Phase, Status: status,
		Stage: "recovered_from_ACT_terminal_marker", Error: detail}
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
	if e := writeJSON(w.path("last_result.json"), r); e != nil {
		return Result{}, false, fmt.Errorf("per-job result recovered, generic result write failed: %w", e)
	}
	return r, true, nil
}
