package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSubmitPersistsLogicalJobAndFirstAttempt(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !validTaskID(task.JobID) || !validTaskID(task.LogicalJobID) || !validTaskID(task.AttemptID) {
		t.Fatalf("expected independent valid job/task/attempt IDs: %+v", task)
	}
	if task.JobID == task.LogicalJobID || task.JobID == task.AttemptID || task.LogicalJobID == task.AttemptID {
		t.Fatalf("IDs should identify distinct entities: %+v", task)
	}
	if task.AttemptNumber != 1 || task.RetryOfJobID != "" {
		t.Fatalf("first attempt metadata incorrect: %+v", task)
	}
	history, err := w.loadTaskHistory(task.LogicalJobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Attempts) != 1 || history.Attempts[0].State != TaskQueued {
		t.Fatalf("first attempt must be durably queued: %+v", history)
	}
	stored, err := readJSON[Task](w.taskAttemptPath(task.AttemptID))
	if err != nil || stored.JobID != task.JobID {
		t.Fatalf("immutable attempt payload missing or mismatched: %+v, %v", stored, err)
	}
}

func TestTaskLifecycleWaitingRunningAndStalledAreDistinct(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.MarkTaskDispatching(task); err != nil {
		t.Fatal(err)
	}
	if attempt, ok, err := w.taskAttemptForTask(task); err != nil || !ok || attempt.State != TaskDispatching {
		t.Fatalf("dispatching state must be persisted before Workbench call: %+v (%v)", attempt, err)
	}
	receipt := DispatchReceipt{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: task.AttemptID, AttemptNumber: task.AttemptNumber, Status: "wb_target_verified"}
	if err := writeJSON(w.path("wb_dispatch_"+task.JobID+".json"), receipt); err != nil {
		t.Fatal(err)
	}
	if state, err := w.refreshTaskAttemptState(task); err != nil || state != TaskWaiting {
		t.Fatalf("Workbench receipt should mean waiting, got %q (%v)", state, err)
	}
	progress := ActProgress{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: task.AttemptID, AttemptNumber: task.AttemptNumber,
		Stage: "正在读取几何", Timestamp: float64(time.Now().Unix())}
	if err := writeJSON(w.path("job_progress_"+task.JobID+".json"), progress); err != nil {
		t.Fatal(err)
	}
	if state, err := w.refreshTaskAttemptState(task); err != nil || state != TaskRunning {
		t.Fatalf("fresh ACT heartbeat should mean running, got %q (%v)", state, err)
	}
	progress.Timestamp = float64(time.Now().Add(-2 * time.Minute).Unix())
	if err := writeJSON(w.path("job_progress_"+task.JobID+".json"), progress); err != nil {
		t.Fatal(err)
	}
	if state, err := w.refreshTaskAttemptState(task); err != nil || state != TaskStalled {
		t.Fatalf("stale heartbeat should mean stalled, got %q (%v)", state, err)
	}
	attempt, ok, err := w.taskAttemptForTask(task)
	if err != nil || !ok {
		t.Fatalf("task attempt history missing: %v", err)
	}
	if attempt.State != TaskStalled || attempt.ResultVerificationStatus != "" {
		t.Fatalf("heartbeat timeout must not be recorded as failure or a result: %+v", attempt)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatalf("stalled task lease must remain: %v", err)
	}
	if _, err := w.RetryTask(task.JobID); err == nil {
		t.Fatal("stalled task must never be retried")
	}
	if err := writeJSON(w.path("last_result.json"), Result{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: task.AttemptID, AttemptNumber: task.AttemptNumber, Phase: task.Phase, Status: "started"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(w.path("job_progress_" + task.JobID + ".json")); err != nil {
		t.Fatal(err)
	}
	if state, err := w.refreshTaskAttemptState(task); err != nil || state != TaskStalled {
		t.Fatalf("a missing heartbeat file must not revive a previously stalled task: %q (%v)", state, err)
	}
	if status, _ := remoteJobObservation(w, task.JobID); status != "act_stalled" {
		t.Fatalf("a generic started receipt must not hide a previously stalled task: %q", status)
	}
}

func TestVerifiedFailureCanCreateLinkedManualRetry(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.MarkTaskDispatching(task); err != nil {
		t.Fatal(err)
	}
	result := Result{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: task.AttemptID, AttemptNumber: task.AttemptNumber,
		Phase: task.Phase, Status: "error", CompletedAt: time.Now().Format(time.RFC3339Nano),
		Error: "fixture failure"}
	if err := writeJSON(w.path("job_"+task.JobID+".json"), result); err != nil {
		t.Fatal(err)
	}
	status, terminal := remoteJobObservation(w, task.JobID)
	if status != "act_terminal" || terminal == nil || terminal.Status != "error" {
		t.Fatalf("expected exact terminal failure receipt, got %q %+v", status, terminal)
	}
	if err := w.releaseTaskLease(task.JobID); err != nil {
		t.Fatal(err)
	}
	retry, err := w.RetryTask(task.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if retry.LogicalJobID != task.LogicalJobID || retry.AttemptNumber != 2 || retry.RetryOfJobID != task.JobID {
		t.Fatalf("retry lineage incorrect: first=%+v retry=%+v", task, retry)
	}
	if retry.JobID == task.JobID || retry.AttemptID == task.AttemptID {
		t.Fatalf("retry must have fresh execution and attempt identities: %+v", retry)
	}
	history, err := w.loadTaskHistory(task.LogicalJobID)
	if err != nil || len(history.Attempts) != 2 {
		t.Fatalf("both attempts must remain in history: %+v, %v", history, err)
	}
	if history.Attempts[0].State != TaskFailed || history.Attempts[1].State != TaskQueued {
		t.Fatalf("unexpected attempt states: %+v", history.Attempts)
	}
	if _, err := os.Stat(w.taskAttemptPath(task.AttemptID)); err != nil {
		t.Fatalf("previous attempt payload must remain: %v", err)
	}
}

func TestPendingVerificationBlocksRetryUntilExplicitConfirmation(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.MarkTaskDispatching(task); err != nil {
		t.Fatal(err)
	}
	progress := ActProgress{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: task.AttemptID, AttemptNumber: task.AttemptNumber,
		Stage: "终态结果全部写入失败", Detail: "result write unavailable",
		Timestamp: float64(time.Now().Add(-20*time.Second).UnixNano()) / 1e9}
	if err := writeJSON(w.path("job_progress_"+task.JobID+".json"), progress); err != nil {
		t.Fatal(err)
	}
	status, terminal := remoteJobObservation(w, task.JobID)
	if status != "act_terminal" || terminal == nil || terminal.VerificationStatus != "pending" {
		t.Fatalf("missing terminal receipt must remain pending verification: %q %+v", status, terminal)
	}
	if err := w.releaseTaskLease(task.JobID); err != nil {
		t.Fatal(err)
	}
	attempt, ok, err := w.taskAttemptForTask(task)
	if err != nil || !ok || attempt.State != TaskResultPendingVerification {
		t.Fatalf("expected pending verification state, got %+v (%v)", attempt, err)
	}
	if _, err := w.RetryTask(task.JobID); err == nil || !strings.Contains(err.Error(), "只有明确核验的失败任务") {
		t.Fatalf("pending result must block retry: %v", err)
	}
	if err := os.Remove(w.path("job_" + task.JobID + ".json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(w.path("last_result.json")); err != nil {
		t.Fatal(err)
	}
	if state, err := w.refreshTaskAttemptState(task); err != nil || state != TaskResultPendingVerification {
		t.Fatalf("pending verification must remain sticky if a recovered receipt is unavailable: %q (%v)", state, err)
	}
	if err := w.VerifyPendingTaskResult(task.JobID, TaskFailed, "已核对当前工程，确认该次操作未完成"); err != nil {
		t.Fatal(err)
	}
	retry, err := w.RetryTask(task.JobID)
	if err != nil {
		t.Fatalf("manual verification should enable an explicit retry: %v", err)
	}
	if retry.AttemptNumber != 2 || retry.RetryOfJobID != task.JobID {
		t.Fatalf("retry lineage lost after manual verification: %+v", retry)
	}
}

func TestLegacyTaskAndReceiptRemainReadable(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	jobID := strings.Repeat("a", 32)
	legacyTask := Task{JobID: jobID, Phase: "inspect"}
	if err := writeJSON(w.taskLeasePath(), legacyTask); err != nil {
		t.Fatal(err)
	}
	legacyResult := Result{JobID: jobID, Phase: "inspect", Status: "ok"}
	if err := writeJSON(w.path("job_"+jobID+".json"), legacyResult); err != nil {
		t.Fatal(err)
	}
	status, result := remoteJobObservation(w, jobID)
	if status != "act_terminal" || result == nil || result.Status != "ok" {
		t.Fatalf("legacy exact-job receipt should remain readable: %q %+v", status, result)
	}
	if err := w.releaseTaskLease(jobID); err != nil {
		t.Fatal(err)
	}
	history, err := w.loadTaskHistory(jobID)
	if err != nil || len(history.Attempts) != 1 || history.Attempts[0].State != TaskSucceeded {
		t.Fatalf("legacy task should gain a compatible history projection: %+v, %v", history, err)
	}
	if _, ok, err := w.taskAttemptForTask(legacyTask); err != nil || !ok {
		t.Fatalf("legacy identity normalization failed: ok=%v err=%v", ok, err)
	}
}

func TestCancelledReceiptIsTerminalButNeverRetryable(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.MarkTaskDispatching(task); err != nil {
		t.Fatal(err)
	}
	result := Result{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: task.AttemptID, AttemptNumber: task.AttemptNumber,
		Phase: task.Phase, Status: "cancelled", CompletedAt: time.Now().Format(time.RFC3339Nano)}
	if err := writeJSON(w.path("job_"+task.JobID+".json"), result); err != nil {
		t.Fatal(err)
	}
	if status, terminal := remoteJobObservation(w, task.JobID); status != "act_terminal" || terminal == nil {
		t.Fatalf("explicit cancellation receipt should be terminal: %q %+v", status, terminal)
	}
	if err := w.releaseTaskLease(task.JobID); err != nil {
		t.Fatal(err)
	}
	attempt, ok, err := w.taskAttemptForTask(task)
	if err != nil || !ok || attempt.State != TaskCancelled || attempt.ResultVerificationStatus != "verified" {
		t.Fatalf("cancelled result should be recorded distinctly: %+v (%v)", attempt, err)
	}
	if _, err := w.RetryTask(task.JobID); err == nil {
		t.Fatal("cancelled tasks must not be retried as failed tasks")
	}
}

func TestTaskStateMachineRejectsTerminalRegression(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.setTaskState(task, TaskSucceeded, taskAttemptObservation{}); err != nil {
		t.Fatal(err)
	}
	if err := w.setTaskState(task, TaskRunning, taskAttemptObservation{}); err == nil {
		t.Fatal("terminal task state must not regress to running")
	}
}

func TestResultIdentityMismatchIsNotAcceptedForCurrentAttempt(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	wrong := Result{JobID: task.JobID, LogicalJobID: task.LogicalJobID,
		AttemptID: strings.Repeat("f", 32), AttemptNumber: task.AttemptNumber,
		Phase: task.Phase, Status: "ok"}
	if err := writeJSON(w.path("job_"+task.JobID+".json"), wrong); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.terminalResultForTask(task); ok {
		t.Fatal("receipt for another attempt must not terminate the current attempt")
	}
	if state, _ := w.refreshTaskAttemptState(task); state == TaskSucceeded {
		t.Fatal("mismatched receipt must not mark the attempt succeeded")
	}
}

func TestTaskHistoryJSONHasStableAttemptAssociation(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(w.taskHistoryPath(task.LogicalJobID))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["logical_job_id"] != task.LogicalJobID {
		t.Fatalf("logical job identity missing from history: %s", b)
	}
}
