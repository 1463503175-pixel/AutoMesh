package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// JobID remains the per-execution identifier used by the existing ACT protocol.
// LogicalJobID groups retries; AttemptID uniquely identifies one execution.
type TaskLifecycleState string

const (
	TaskQueued                    TaskLifecycleState = "queued"
	TaskDispatching               TaskLifecycleState = "dispatching"
	TaskRunning                   TaskLifecycleState = "running"
	TaskWaiting                   TaskLifecycleState = "waiting"
	TaskStalled                   TaskLifecycleState = "stalled"
	TaskResultPendingVerification TaskLifecycleState = "result_pending_verification"
	TaskSucceeded                 TaskLifecycleState = "succeeded"
	TaskFailed                    TaskLifecycleState = "failed"
	TaskCancelled                 TaskLifecycleState = "cancelled"
)

func taskStateLabel(state TaskLifecycleState) string {
	switch state {
	case TaskQueued:
		return "排队中"
	case TaskDispatching:
		return "正在投递"
	case TaskRunning:
		return "运行中"
	case TaskWaiting:
		return "等待 Meshing 确认"
	case TaskStalled:
		return "心跳异常（未确认停止）"
	case TaskResultPendingVerification:
		return "结果待核验"
	case TaskSucceeded:
		return "成功"
	case TaskFailed:
		return "失败"
	case TaskCancelled:
		return "已取消"
	default:
		return "状态未知"
	}
}

type TaskAttemptRecord struct {
	LogicalJobID             string             `json:"logical_job_id"`
	JobID                    string             `json:"job_id"`
	AttemptID                string             `json:"attempt_id"`
	AttemptNumber            int                `json:"attempt_number"`
	RetryOfJobID             string             `json:"retry_of_job_id,omitempty"`
	Phase                    string             `json:"phase"`
	MeshSystem               string             `json:"mesh_system,omitempty"`
	State                    TaskLifecycleState `json:"state"`
	SubmittedAt              string             `json:"submitted_at"`
	UpdatedAt                string             `json:"updated_at"`
	DispatchStartedAt        string             `json:"dispatch_started_at,omitempty"`
	LastHeartbeatAt          string             `json:"last_heartbeat_at,omitempty"`
	ResultCompletedAt        string             `json:"result_completed_at,omitempty"`
	ResultStatus             string             `json:"result_status,omitempty"`
	ResultVerificationStatus string             `json:"result_verification_status,omitempty"`
	ResultVerificationNote   string             `json:"result_verification_note,omitempty"`
	Error                    string             `json:"error,omitempty"`
}

type TaskHistory struct {
	SchemaVersion int                 `json:"schema_version"`
	LogicalJobID  string              `json:"logical_job_id"`
	Phase         string              `json:"phase"`
	MeshSystem    string              `json:"mesh_system,omitempty"`
	CreatedAt     string              `json:"created_at"`
	UpdatedAt     string              `json:"updated_at"`
	Attempts      []TaskAttemptRecord `json:"attempts"`
}

var taskHistoryUpdateMu sync.Mutex

type taskAttemptObservation struct {
	HeartbeatAt   time.Time
	DispatchStart time.Time
	Result        *Result
	ManualNote    string
}

func normalizeTaskIdentity(t Task) Task {
	if t.LogicalJobID == "" {
		t.LogicalJobID = t.JobID
	}
	if t.AttemptID == "" {
		t.AttemptID = t.JobID
	}
	if t.AttemptNumber <= 0 {
		t.AttemptNumber = 1
	}
	return t
}

func (w Workspace) taskHistoryPath(logicalJobID string) string {
	return w.path("task_history_" + logicalJobID + ".json")
}

func (w Workspace) taskAttemptPath(attemptID string) string {
	return w.path("attempt_" + attemptID + ".json")
}

func validTaskID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func taskAttemptRecord(t Task, state TaskLifecycleState) TaskAttemptRecord {
	t = normalizeTaskIdentity(t)
	return TaskAttemptRecord{
		LogicalJobID: t.LogicalJobID, JobID: t.JobID, AttemptID: t.AttemptID,
		AttemptNumber: t.AttemptNumber, RetryOfJobID: t.RetryOfJobID,
		Phase: t.Phase, MeshSystem: t.MeshSystem, State: state,
		SubmittedAt: t.SubmittedAt, UpdatedAt: time.Now().Format(time.RFC3339Nano),
	}
}

func (w Workspace) loadTaskHistory(logicalJobID string) (TaskHistory, error) {
	var history TaskHistory
	if !validTaskID(logicalJobID) {
		return history, errors.New("逻辑任务 ID 无效")
	}
	err := readJSONFile(w.taskHistoryPath(logicalJobID), &history)
	if err != nil {
		return history, err
	}
	if history.LogicalJobID != logicalJobID || history.SchemaVersion < 1 {
		return TaskHistory{}, errors.New("任务历史身份或版本不匹配；拒绝覆盖")
	}
	return history, nil
}

// readJSONFile is kept local to the lifecycle model so a malformed history
// cannot be mistaken for a missing file and silently replaced.
func readJSONFile(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (w Workspace) writeTaskHistory(history TaskHistory) error {
	if !validTaskID(history.LogicalJobID) || history.SchemaVersion != 1 {
		return errors.New("任务历史记录身份无效")
	}
	history.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	return writeJSON(w.taskHistoryPath(history.LogicalJobID), history)
}

func (w Workspace) initializeTaskAttempt(t Task) error {
	t = normalizeTaskIdentity(t)
	if !validTaskID(t.JobID) || !validTaskID(t.LogicalJobID) || !validTaskID(t.AttemptID) || t.Phase == "" || t.AttemptNumber < 1 {
		return errors.New("任务或执行尝试身份无效")
	}
	attemptPath := w.taskAttemptPath(t.AttemptID)
	if _, err := os.Stat(attemptPath); err == nil {
		return errors.New("执行尝试 ID 已存在；拒绝覆盖历史记录")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	history, err := w.loadTaskHistory(t.LogicalJobID)
	if errors.Is(err, os.ErrNotExist) {
		if t.AttemptNumber != 1 || t.RetryOfJobID != "" {
			return errors.New("新逻辑任务的首次执行尝试编号必须为 1")
		}
		now := time.Now().Format(time.RFC3339Nano)
		history = TaskHistory{SchemaVersion: 1, LogicalJobID: t.LogicalJobID,
			Phase: t.Phase, MeshSystem: t.MeshSystem, CreatedAt: now, UpdatedAt: now,
			Attempts: []TaskAttemptRecord{}}
	} else if err != nil {
		return fmt.Errorf("读取任务历史失败：%w", err)
	} else {
		if t.AttemptNumber != len(history.Attempts)+1 || len(history.Attempts) == 0 {
			return errors.New("执行尝试编号与历史记录不连续")
		}
		previous := history.Attempts[len(history.Attempts)-1]
		if t.RetryOfJobID == "" || t.RetryOfJobID != previous.JobID {
			return errors.New("重试必须明确关联最近一次执行的 JobID")
		}
		if previous.State != TaskFailed || (previous.ResultVerificationStatus != "verified" && previous.ResultVerificationStatus != "manually_verified") {
			return errors.New("上一次执行尚未核验为明确失败，禁止创建重试尝试")
		}
	}

	if err := writeJSON(attemptPath, t); err != nil {
		return fmt.Errorf("保存执行尝试快照失败：%w", err)
	}
	historyPersisted := false
	defer func() {
		if !historyPersisted {
			_ = os.Remove(attemptPath)
		}
	}()
	history.Attempts = append(history.Attempts, taskAttemptRecord(t, TaskQueued))
	if err := w.writeTaskHistory(history); err != nil {
		return fmt.Errorf("保存任务历史失败：%w", err)
	}
	historyPersisted = true
	if err := writeJSON(w.path("job.json"), t); err != nil {
		_ = w.updateTaskAttempt(t, func(record *TaskAttemptRecord) {
			record.State = TaskFailed
			record.ResultStatus = "local_submission_error"
			record.ResultVerificationStatus = "verified"
			record.Error = "任务尚未投递；本地 job.json 写入失败：" + err.Error()
		})
		return fmt.Errorf("写入 ACT 任务文件失败（尚未投递）：%w", err)
	}
	return nil
}

func (w Workspace) updateTaskAttempt(t Task, mutate func(*TaskAttemptRecord)) error {
	taskHistoryUpdateMu.Lock()
	defer taskHistoryUpdateMu.Unlock()
	t = normalizeTaskIdentity(t)
	if !validTaskID(t.LogicalJobID) || !validTaskID(t.JobID) || !validTaskID(t.AttemptID) {
		return errors.New("任务尝试身份无效")
	}
	history, err := w.loadTaskHistory(t.LogicalJobID)
	if errors.Is(err, os.ErrNotExist) {
		now := time.Now().Format(time.RFC3339Nano)
		history = TaskHistory{SchemaVersion: 1, LogicalJobID: t.LogicalJobID,
			Phase: t.Phase, MeshSystem: t.MeshSystem, CreatedAt: now, UpdatedAt: now,
			Attempts: []TaskAttemptRecord{taskAttemptRecord(t, TaskQueued)}}
	} else if err != nil {
		return fmt.Errorf("读取任务历史失败：%w", err)
	}
	index := -1
	for i := range history.Attempts {
		if history.Attempts[i].JobID == t.JobID {
			index = i
			break
		}
	}
	if index < 0 {
		// Legacy active_task.json files predate the history model. Add a
		// compatibility record keyed to their existing job ID; never rewrite
		// the old task or receipt payload.
		history.Attempts = append(history.Attempts, taskAttemptRecord(t, TaskQueued))
		index = len(history.Attempts) - 1
	}
	record := &history.Attempts[index]
	if record.AttemptID != t.AttemptID || record.LogicalJobID != t.LogicalJobID {
		return errors.New("任务历史中的执行尝试身份不匹配；拒绝更新")
	}
	mutate(record)
	record.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	return w.writeTaskHistory(history)
}

func taskStateTransitionAllowed(from, to TaskLifecycleState) bool {
	if from == to || from == "" {
		return true
	}
	switch from {
	case TaskQueued:
		return to == TaskDispatching || to == TaskRunning || to == TaskWaiting || to == TaskStalled || to == TaskResultPendingVerification || to == TaskSucceeded || to == TaskFailed || to == TaskCancelled
	case TaskDispatching:
		return to == TaskWaiting || to == TaskRunning || to == TaskStalled || to == TaskResultPendingVerification || to == TaskSucceeded || to == TaskFailed || to == TaskCancelled
	case TaskWaiting:
		return to == TaskRunning || to == TaskStalled || to == TaskResultPendingVerification || to == TaskSucceeded || to == TaskFailed || to == TaskCancelled
	case TaskRunning:
		return to == TaskWaiting || to == TaskStalled || to == TaskResultPendingVerification || to == TaskSucceeded || to == TaskFailed || to == TaskCancelled
	case TaskStalled:
		return to == TaskWaiting || to == TaskRunning || to == TaskResultPendingVerification || to == TaskSucceeded || to == TaskFailed || to == TaskCancelled
	case TaskResultPendingVerification:
		return to == TaskSucceeded || to == TaskFailed || to == TaskCancelled
	default:
		return false
	}
}

func (w Workspace) setTaskState(t Task, next TaskLifecycleState, observation taskAttemptObservation) error {
	if next != TaskQueued && next != TaskDispatching && next != TaskRunning && next != TaskWaiting && next != TaskStalled && next != TaskResultPendingVerification && next != TaskSucceeded && next != TaskFailed && next != TaskCancelled {
		return fmt.Errorf("未知任务状态：%q", next)
	}
	t = normalizeTaskIdentity(t)
	current := TaskQueued
	if record, ok, err := w.taskAttemptForTask(t); err == nil && ok {
		current = record.State
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !taskStateTransitionAllowed(current, next) {
		return fmt.Errorf("不允许任务状态从 %s 转换为 %s", current, next)
	}
	if existing, ok, err := w.taskAttemptForTask(t); err == nil && ok && existing.State == next {
		unchanged := true
		if !observation.DispatchStart.IsZero() && existing.DispatchStartedAt == "" {
			unchanged = false
		}
		if !observation.HeartbeatAt.IsZero() && existing.LastHeartbeatAt != observation.HeartbeatAt.Format(time.RFC3339Nano) {
			unchanged = false
		}
		if observation.Result != nil {
			r := observation.Result
			verification := "verified"
			if r.VerificationStatus == "pending" || next == TaskResultPendingVerification {
				verification = "pending"
			}
			if existing.ResultStatus != r.Status || existing.ResultCompletedAt != r.CompletedAt || existing.ResultVerificationStatus != verification || existing.Error != r.Error {
				unchanged = false
			}
		}
		if observation.ManualNote != "" && (existing.ResultVerificationStatus != "manually_verified" || existing.ResultVerificationNote != observation.ManualNote) {
			unchanged = false
		}
		if unchanged {
			return nil
		}
	}
	return w.updateTaskAttempt(t, func(record *TaskAttemptRecord) {
		record.State = next
		if !observation.DispatchStart.IsZero() && record.DispatchStartedAt == "" {
			record.DispatchStartedAt = observation.DispatchStart.Format(time.RFC3339Nano)
		}
		if !observation.HeartbeatAt.IsZero() {
			record.LastHeartbeatAt = observation.HeartbeatAt.Format(time.RFC3339Nano)
		}
		if observation.Result != nil {
			r := observation.Result
			record.ResultStatus = r.Status
			record.ResultCompletedAt = r.CompletedAt
			record.Error = r.Error
			if r.VerificationStatus == "pending" || next == TaskResultPendingVerification {
				record.ResultVerificationStatus = "pending"
			} else {
				record.ResultVerificationStatus = "verified"
			}
		}
		if observation.ManualNote != "" {
			record.ResultVerificationStatus = "manually_verified"
			record.ResultVerificationNote = observation.ManualNote
		}
	})
}

func (w Workspace) taskAttemptForTask(t Task) (TaskAttemptRecord, bool, error) {
	t = normalizeTaskIdentity(t)
	history, err := w.loadTaskHistory(t.LogicalJobID)
	if err != nil {
		return TaskAttemptRecord{}, false, err
	}
	for _, attempt := range history.Attempts {
		if attempt.JobID != t.JobID {
			continue
		}
		if attempt.AttemptID != t.AttemptID || attempt.LogicalJobID != t.LogicalJobID || attempt.AttemptNumber != t.AttemptNumber || attempt.RetryOfJobID != t.RetryOfJobID {
			return TaskAttemptRecord{}, false, errors.New("任务历史与当前执行尝试元数据不一致")
		}
		return attempt, true, nil
	}
	return TaskAttemptRecord{}, false, nil
}

func taskStateFromResult(result Result) TaskLifecycleState {
	if result.VerificationStatus == "pending" || result.Stage == "recovered_from_ACT_terminal_marker" {
		return TaskResultPendingVerification
	}
	switch result.Status {
	case "ok":
		return TaskSucceeded
	case "error":
		return TaskFailed
	case "cancelled":
		return TaskCancelled
	default:
		return ""
	}
}

func resultMatchesTask(t Task, result Result) bool {
	t = normalizeTaskIdentity(t)
	if result.JobID != t.JobID {
		return false
	}
	if result.LogicalJobID != "" && result.LogicalJobID != t.LogicalJobID {
		return false
	}
	if result.AttemptID != "" && result.AttemptID != t.AttemptID {
		return false
	}
	if result.AttemptNumber > 0 && result.AttemptNumber != t.AttemptNumber {
		return false
	}
	if result.Phase != "" && t.Phase != "" && result.Phase != t.Phase {
		return false
	}
	return true
}

func (w Workspace) terminalResultForTask(t Task) (Result, bool) {
	t = normalizeTaskIdentity(t)
	result, ok := w.terminalResultForJob(t.JobID)
	if !ok || !resultMatchesTask(t, result) {
		return Result{}, false
	}
	return result, true
}

// refreshTaskAttemptState updates only local history from durable ACT/Workbench
// receipts. A stale heartbeat is represented as stalled, never as failed.
func (w Workspace) refreshTaskAttemptState(t Task) (TaskLifecycleState, error) {
	t = normalizeTaskIdentity(t)
	persistableIdentity := validTaskID(t.LogicalJobID) && validTaskID(t.JobID) && validTaskID(t.AttemptID)
	var current TaskAttemptRecord
	var hasCurrent bool
	if persistableIdentity {
		current, hasCurrent, _ = w.taskAttemptForTask(t)
		if hasCurrent && (current.State == TaskSucceeded || current.State == TaskFailed || current.State == TaskCancelled) {
			return current.State, nil
		}
	}
	if result, ok := w.terminalResultForTask(t); ok {
		state := taskStateFromResult(result)
		if state == "" {
			return "", nil
		}
		if persistableIdentity {
			if err := w.setTaskState(t, state, taskAttemptObservation{Result: &result}); err != nil {
				return state, err
			}
		}
		return state, nil
	}
	if hasCurrent && current.State == TaskResultPendingVerification {
		return TaskResultPendingVerification, nil
	}
	progress, progressErr := readJSON[ActProgress](w.path("job_progress_" + t.JobID + ".json"))
	if progressErr == nil && progress.JobID == t.JobID &&
		(progress.AttemptID == "" || progress.AttemptID == t.AttemptID) &&
		(progress.LogicalJobID == "" || progress.LogicalJobID == t.LogicalJobID) {
		if progress.Timestamp > 0 {
			heartbeat := time.Unix(int64(progress.Timestamp), 0)
			state := TaskRunning
			if time.Since(heartbeat) > 90*time.Second {
				state = TaskStalled
			}
			if persistableIdentity {
				if err := w.setTaskState(t, state, taskAttemptObservation{HeartbeatAt: heartbeat}); err != nil {
					return state, err
				}
			}
			return state, nil
		}
		if persistableIdentity {
			if err := w.setTaskState(t, TaskRunning, taskAttemptObservation{}); err != nil {
				return TaskRunning, err
			}
		}
		return TaskRunning, nil
	}
	if hasCurrent && current.State == TaskStalled {
		return TaskStalled, nil
	}
	if running, err := w.LoadResult(); err == nil && running.JobID == t.JobID && resultMatchesTask(t, running) &&
		(running.Status == "started" || running.Status == "in_progress") {
		if persistableIdentity {
			if err := w.setTaskState(t, TaskRunning, taskAttemptObservation{}); err != nil {
				return TaskRunning, err
			}
		}
		return TaskRunning, nil
	}
	dispatch, dispatchErr := readJSON[DispatchReceipt](w.path("wb_dispatch_" + t.JobID + ".json"))
	if dispatchErr == nil && dispatch.JobID == t.JobID &&
		(dispatch.AttemptID == "" || dispatch.AttemptID == t.AttemptID) &&
		(dispatch.LogicalJobID == "" || dispatch.LogicalJobID == t.LogicalJobID) {
		if persistableIdentity {
			if err := w.setTaskState(t, TaskWaiting, taskAttemptObservation{}); err != nil {
				return TaskWaiting, err
			}
		}
		return TaskWaiting, nil
	}
	if persistableIdentity {
		attempt, ok, err := w.taskAttemptForTask(t)
		if err != nil {
			return "", err
		}
		if ok {
			return attempt.State, nil
		}
	}
	return TaskQueued, nil
}

func (w Workspace) MarkTaskDispatching(t Task) error {
	now := time.Now()
	return w.setTaskState(t, TaskDispatching, taskAttemptObservation{DispatchStart: now})
}

func (w Workspace) recordTaskResult(t Task, result Result) error {
	if !resultMatchesTask(t, result) {
		return errors.New("任务结果与当前执行尝试身份不匹配")
	}
	state := taskStateFromResult(result)
	if state == "" {
		return fmt.Errorf("无法从结果状态 %q 推导任务终态", result.Status)
	}
	return w.setTaskState(t, state, taskAttemptObservation{Result: &result})
}

// VerifyPendingTaskResult is an explicit operator confirmation path for an
// ACT terminal marker whose JSON receipt could not establish the outcome.
// It never contacts ANSYS and never edits the original immutable result file.
func (w Workspace) VerifyPendingTaskResult(jobID string, outcome TaskLifecycleState, note string) error {
	if lease, err := readJSON[Task](w.taskLeasePath()); err == nil && lease.JobID == jobID {
		return errors.New("目标任务仍持有活动占用记录；先确认终态已安全释放，再进行结果核验")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取活动任务占用失败：%w", err)
	}
	if outcome != TaskSucceeded && outcome != TaskFailed && outcome != TaskCancelled {
		return errors.New("核验结果必须明确为 succeeded、failed 或 cancelled")
	}
	if strings.TrimSpace(note) == "" {
		return errors.New("人工核验必须记录依据说明")
	}
	t, _, err := w.findTaskByJobID(jobID)
	if err != nil {
		return err
	}
	attempt, ok, err := w.taskAttemptForTask(t)
	if err != nil || !ok {
		return errors.New("找不到目标执行尝试的任务历史")
	}
	if attempt.State != TaskResultPendingVerification {
		return fmt.Errorf("当前任务状态为 %s，不需要结果待核验确认", attempt.State)
	}
	return w.setTaskState(t, outcome, taskAttemptObservation{ManualNote: strings.TrimSpace(note)})
}

func (w Workspace) findTaskByJobID(jobID string) (Task, TaskAttemptRecord, error) {
	if !validTaskID(jobID) {
		return Task{}, TaskAttemptRecord{}, errors.New("JobID 无效")
	}
	if lease, err := readJSON[Task](w.taskLeasePath()); err == nil && lease.JobID == jobID {
		lease = normalizeTaskIdentity(lease)
		attempt, ok, historyErr := w.taskAttemptForTask(lease)
		if historyErr == nil && ok {
			return lease, attempt, nil
		}
	}
	files, err := filepath.Glob(w.path("task_history_*.json"))
	if err != nil {
		return Task{}, TaskAttemptRecord{}, err
	}
	for _, file := range files {
		var history TaskHistory
		if err := readJSONFile(file, &history); err != nil || !validTaskID(history.LogicalJobID) {
			continue
		}
		for _, attempt := range history.Attempts {
			if attempt.JobID != jobID {
				continue
			}
			if !validTaskID(attempt.AttemptID) || !validTaskID(attempt.LogicalJobID) {
				return Task{}, TaskAttemptRecord{}, errors.New("任务历史中的执行尝试 ID 无效")
			}
			t, readErr := readJSON[Task](w.taskAttemptPath(attempt.AttemptID))
			if readErr == nil && t.JobID == jobID {
				t = normalizeTaskIdentity(t)
				if t.LogicalJobID != attempt.LogicalJobID || t.AttemptID != attempt.AttemptID || t.AttemptNumber != attempt.AttemptNumber || t.RetryOfJobID != attempt.RetryOfJobID {
					return Task{}, TaskAttemptRecord{}, errors.New("执行尝试载荷与任务历史身份不一致")
				}
				return t, attempt, nil
			}
			if !errors.Is(readErr, os.ErrNotExist) && readErr != nil {
				return Task{}, TaskAttemptRecord{}, fmt.Errorf("读取执行尝试原始载荷失败：%w", readErr)
			}
			// Old tasks may have only active_task/job receipts and no immutable
			// attempt payload. This reconstructed metadata is sufficient for
			// manual result verification, but RetryTask separately requires the
			// original payload and will refuse to clone an incomplete task.
			legacy := Task{JobID: attempt.JobID, LogicalJobID: attempt.LogicalJobID,
				AttemptID: attempt.AttemptID, AttemptNumber: attempt.AttemptNumber,
				RetryOfJobID: attempt.RetryOfJobID, Phase: attempt.Phase,
				MeshSystem: attempt.MeshSystem, SubmittedAt: attempt.SubmittedAt}
			return normalizeTaskIdentity(legacy), attempt, nil
		}
	}
	return Task{}, TaskAttemptRecord{}, errors.New("找不到该 JobID 对应的任务历史")
}

// RetryTask creates a new, explicitly linked attempt only after a verified
// failure. It is never called automatically by timeout or heartbeat logic.
func (w Workspace) RetryTask(previousJobID string) (Task, error) {
	if _, err := os.Stat(w.taskLeasePath()); err == nil {
		return Task{}, errors.New("当前仍存在任务占用记录；先按现有安全流程核实并处理，不创建重试")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Task{}, err
	}
	previous, previousRecord, err := w.findTaskByJobID(previousJobID)
	if err != nil {
		return Task{}, err
	}
	persistedPayload, payloadErr := readJSON[Task](w.taskAttemptPath(previousRecord.AttemptID))
	if payloadErr != nil || persistedPayload.JobID != previousJobID {
		return Task{}, errors.New("缺少原执行尝试的完整提交载荷；为避免丢失 Plan 或参数，不允许重试旧版任务")
	}
	previous = normalizeTaskIdentity(persistedPayload)
	if previous.LogicalJobID != previousRecord.LogicalJobID || previous.AttemptID != previousRecord.AttemptID || previous.AttemptNumber != previousRecord.AttemptNumber || previous.RetryOfJobID != previousRecord.RetryOfJobID {
		return Task{}, errors.New("原执行尝试载荷与任务历史不一致；拒绝重试")
	}
	if previousRecord.State != TaskFailed || (previousRecord.ResultVerificationStatus != "verified" && previousRecord.ResultVerificationStatus != "manually_verified") {
		return Task{}, fmt.Errorf("上一次执行状态为 %s（核验状态：%s）；只有明确核验的失败任务才允许人工创建重试", previousRecord.State, previousRecord.ResultVerificationStatus)
	}
	if result, ok := w.terminalResultForTask(previous); ok {
		if result.Status != "error" && previousRecord.ResultVerificationStatus != "manually_verified" {
			return Task{}, errors.New("执行尝试结果不是明确失败；拒绝重试")
		}
	} else if previousRecord.ResultVerificationStatus != "manually_verified" {
		return Task{}, errors.New("缺少与该执行尝试身份匹配的失败回执；需要人工核验后才可重试")
	}
	next := previous
	next.JobID = newID()
	next.AttemptID = newID()
	next.AttemptNumber = previousRecord.AttemptNumber + 1
	next.RetryOfJobID = previous.JobID
	next.SubmittedAt = time.Now().Format(time.RFC3339Nano)
	if err := w.acquireTaskLease(next); err != nil {
		return Task{}, err
	}
	if err := w.initializeTaskAttempt(next); err != nil {
		_ = w.removeTaskLeaseIfOwned(next.JobID)
		return Task{}, err
	}
	return next, nil
}

func (w Workspace) removeTaskLeaseIfOwned(jobID string) error {
	lease, err := readJSON[Task](w.taskLeasePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if lease.JobID != jobID {
		return errors.New("任务占用记录已变化；拒绝移除")
	}
	return os.Remove(w.taskLeasePath())
}
