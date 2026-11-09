package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type DispatchReceipt struct {
	JobID         string  `json:"job_id"`
	LogicalJobID  string  `json:"logical_job_id,omitempty"`
	AttemptID     string  `json:"attempt_id,omitempty"`
	AttemptNumber int     `json:"attempt_number,omitempty"`
	RetryOfJobID  string  `json:"retry_of_job_id,omitempty"`
	Status        string  `json:"status"`
	Stage         string  `json:"stage"`
	Timestamp     float64 `json:"timestamp"`
}

const taskStatusRefreshInterval = 2 * time.Second

func taskStatusRefreshDue(last, now time.Time) bool {
	return last.IsZero() || now.Before(last) || now.Sub(last) >= taskStatusRefreshInterval
}

func (w Workspace) taskStatus() string {
	lease, err := readJSON[Task](w.taskLeasePath())
	if err != nil {
		if os.IsNotExist(err) {
			if last, e := w.LoadResult(); e == nil && last.JobID != "" {
				if task, attempt, findErr := w.findTaskByJobID(last.JobID); findErr == nil {
					lines := []string{"当前没有待确认任务占用。", "上次任务：" + task.Phase + "（" + task.JobID + "）",
						"执行尝试：" + attempt.AttemptID + "（第 " + fmt.Sprint(attempt.AttemptNumber) + " 次）",
						"任务状态：" + taskStateLabel(attempt.State) + "（" + string(attempt.State) + "）"}
					if attempt.State == TaskResultPendingVerification {
						lines = append(lines, "结果尚未核验；禁止自动重试。请先核对当前工程状态并完成结果确认。")
						if last.Stage == "recovered_from_ACT_terminal_marker" {
							lines = append(lines, "已从 ACT 明确的失败/取消信号恢复终态执行记录；原因："+last.Error)
						}
					} else if attempt.ResultVerificationStatus == "manually_verified" {
						lines = append(lines, "结果已人工确认："+attempt.ResultVerificationNote)
					}
					if last.Stage == "recovered_from_job_bound_warning_plan" {
						lines = append(lines, "已从同一 Job ID 的局部方案文件恢复预览结果；未修改 Mesh，也没有重新投递。")
					} else if last.Stage == "recovered_from_ACT_terminal_marker" {
						lines = append(lines, "已从 ACT 明确的失败/取消信号恢复终态执行记录；恢复标记不等于结果已核验。原因："+last.Error)
					}
					return strings.Join(lines, "\n")
				}
				if last.VerificationStatus == "pending" {
					message := "当前没有待确认任务占用。\n上次任务结果待核验：" + last.JobID + "\n原因：" + last.Error + "\n禁止自动重试；未操作 ANSYS。"
					if last.Stage == "recovered_from_ACT_terminal_marker" {
						message += "\n已从 ACT 明确的失败/取消信号恢复终态执行记录。"
					}
					return message
				}
			}
			return "当前没有待确认的 AutoMesh 任务占用。"
		}
		return "任务占用记录不能读取：" + err.Error()
	}
	state, stateErr := w.refreshTaskAttemptState(lease)
	if stateErr != nil {
		state = ""
	}
	lines := []string{"任务 ID：" + lease.JobID, "操作：" + lease.Phase}
	if state != "" {
		lines = append(lines, "统一任务状态："+taskStateLabel(state)+"（"+string(state)+"）")
	}
	if lease.LogicalJobID != "" || lease.AttemptID != "" {
		lines = append(lines, fmt.Sprintf("逻辑任务：%s；执行尝试：%s（第 %d 次）", lease.LogicalJobID, lease.AttemptID, maxInt(lease.AttemptNumber, 1)))
	}
	if lease.RetryOfJobID != "" {
		lines = append(lines, "重试来源 JobID："+lease.RetryOfJobID)
	}
	if lease.MeshSystem != "" {
		lines = append(lines, "Mesh 系统："+lease.MeshSystem)
	}
	if lease.SubmittedAt != "" {
		if started, parseErr := time.Parse(time.RFC3339, lease.SubmittedAt); parseErr == nil {
			lines = append(lines, "开始时间："+started.Format("2006-01-02 15:04:05"))
			lines = append(lines, fmt.Sprintf("运行时长：%s", time.Since(started).Round(time.Second)))
		}
	}
	if history, historyErr := w.loadTaskHistory(normalizeTaskIdentity(lease).LogicalJobID); historyErr == nil && len(history.Attempts) > 0 {
		parts := make([]string, 0, len(history.Attempts))
		for _, item := range history.Attempts {
			shortID := item.JobID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			parts = append(parts, fmt.Sprintf("第%d次 %s（%s）", item.AttemptNumber, shortID, taskStateLabel(item.State)))
		}
		lines = append(lines, "执行尝试历史："+strings.Join(parts, "；"))
	}
	if attempt, ok, historyErr := w.taskAttemptForTask(lease); historyErr == nil && ok {
		if attempt.ResultVerificationStatus == "pending" {
			lines = append(lines, "结果待核验；禁止自动重试")
		} else if attempt.ResultVerificationStatus == "manually_verified" {
			lines = append(lines, "结果核验：已人工确认；依据："+attempt.ResultVerificationNote)
		} else if attempt.ResultVerificationStatus == "verified" && attempt.ResultStatus != "" {
			lines = append(lines, "任务回执：身份已核验（"+attempt.ResultStatus+"）")
		}
	}
	if req, e := readJSON[TaskCancelRequest](w.cancelPath(lease.JobID)); e == nil && req.JobID == lease.JobID {
		lines = append(lines, "已请求安全停止："+req.RequestedAt+"；尚不代表 Meshing 已停止")
	}
	receipt, e := readJSON[DispatchReceipt](w.path("wb_dispatch_" + lease.JobID + ".json"))
	if e == nil && receipt.JobID == lease.JobID && (receipt.AttemptID == "" || receipt.AttemptID == normalizeTaskIdentity(lease).AttemptID) && (receipt.LogicalJobID == "" || receipt.LogicalJobID == normalizeTaskIdentity(lease).LogicalJobID) {
		lines = append(lines, "Workbench 路由："+receipt.Status+"（不代表 Meshing 已运行）")
	} else {
		lines = append(lines, "Workbench 路由：未获确认")
	}
	progress, e := readJSON[ActProgress](w.path("job_progress_" + lease.JobID + ".json"))
	if e == nil && progress.JobID == lease.JobID && (progress.AttemptID == "" || progress.AttemptID == normalizeTaskIdentity(lease).AttemptID) && (progress.LogicalJobID == "" || progress.LogicalJobID == normalizeTaskIdentity(lease).LogicalJobID) {
		lines = append(lines, "Meshing 阶段："+progress.Stage+"；"+progress.Detail)
		if progress.Timestamp > 0 {
			heartbeatAt := time.Unix(int64(progress.Timestamp), 0)
			lines = append(lines, "最近心跳："+heartbeatAt.Format("2006-01-02 15:04:05"))
			if age := time.Since(heartbeatAt); age > 90*time.Second {
				lines = append(lines, fmt.Sprintf("心跳超时：%s 未收到新心跳；这不代表 Meshing 已停止，任务锁继续保留。", age.Round(time.Second)))
			}
		}
		if progress.ProcessPID > 0 {
			lines = append(lines, fmt.Sprintf("ACT 宿主 PID：%d（读取自同一任务回执）", progress.ProcessPID))
			life, reason := inspectMeshingProcess(progress.ProcessPID, progress.ProcessStartedFiletime)
			lines = append(lines, "宿主进程状态："+life+"；"+reason)
		}
	} else {
		lines = append(lines, "Meshing 阶段：未获确认")
	}
	result, terminal := w.terminalResultForTask(lease)
	if terminal {
		lines = append(lines, "ACT 结果："+result.Status+"；"+result.Stage)
	} else {
		lines = append(lines, "ACT 结果：未获当前任务结果")
	}
	// A successful ACT action might have failed to save its JSON receipt.
	// Surface the raw write error to the operator, but NEVER infer that a
	// Sizing was (or was not) committed from that filesystem error.
	if b, readErr := os.ReadFile(w.path("job_receipt_write_error_" + lease.JobID + ".txt")); readErr == nil {
		message := strings.TrimSpace(string(b))
		if len(message) > 900 {
			message = message[:900] + "…"
		}
		lines = append(lines, "任务回执文件写入异常："+message+"；发布结果须核查 Meshing 树")
	}
	lines = append(lines, "仅观察状态，不清锁、不重新投递、不操作网格。")
	return strings.Join(lines, "\n")
}

// A missing or unreadable PID never means a crash. Only the actual Meshing
// process identity reported by this exact ACT job can prove its exit.
func (w Workspace) meshingProcessCrash(jobID string) (string, bool) {
	p, err := readJSON[ActProgress](w.path("job_progress_" + jobID + ".json"))
	if err != nil || p.JobID != jobID || p.ProcessPID <= 0 || p.ProcessStartedFiletime == "" {
		return "", false
	}
	status, reason := inspectMeshingProcess(p.ProcessPID, p.ProcessStartedFiletime)
	if status != "exited" {
		return "", false
	}
	return fmt.Sprintf("ACT 任务 %s 原 Meshing 进程 PID %d 已退出（%s）", jobID[:8], p.ProcessPID, reason), true
}

// Use while waiting on a synchronous Workbench SendCommand transport.
// A timeout cannot prove the remote ACT task is stopped.
func remoteJobObservation(w Workspace, jobID string) (status string, terminal *Result) {
	// ACT may have completed its operation but failed to write its final JSON.
	// Recover only when this exact job has an explicit terminal-write marker;
	// do not release the lease here. taskRun owns release after observing result.
	lease, leaseErr := readJSON[Task](w.taskLeasePath())
	if leaseErr == nil && lease.JobID == jobID {
		if progress, progressErr := readJSON[ActProgress](w.path("job_progress_" + jobID + ".json")); progressErr == nil && progress.JobID == jobID &&
			(progress.AttemptID == "" || progress.AttemptID == normalizeTaskIdentity(lease).AttemptID) &&
			(progress.LogicalJobID == "" || progress.LogicalJobID == normalizeTaskIdentity(lease).LogicalJobID) &&
			(progress.Stage == "终态结果全部写入失败" || progress.Stage == "部分回执写入失败" || progress.Stage == "任务失败" || progress.Stage == "已安全取消") {
			_, _, _ = w.recoverTerminalProgress(lease)
		}
		_, _ = w.refreshTaskAttemptState(lease)
		if result, ok := w.terminalResultForTask(lease); ok {
			return "act_terminal", &result
		}
	} else if result, ok := w.terminalResultForJob(jobID); ok {
		return "act_terminal", &result
	}
	// A stale exact-job heartbeat takes precedence over a generic started or
	// in_progress result. It is still only a stalled observation, not failure.
	if p, e := readJSON[ActProgress](w.path("job_progress_" + jobID + ".json")); e == nil && p.JobID == jobID &&
		(leaseErr != nil || ((p.AttemptID == "" || p.AttemptID == normalizeTaskIdentity(lease).AttemptID) && (p.LogicalJobID == "" || p.LogicalJobID == normalizeTaskIdentity(lease).LogicalJobID))) {
		if p.Timestamp > 0 && time.Since(time.Unix(int64(p.Timestamp), 0)) > 90*time.Second {
			return "act_stalled", nil // Lack of a heartbeat is not evidence that ACT exited.
		}
		return "act_running", nil
	}
	if leaseErr == nil && lease.JobID == jobID {
		if attempt, ok, err := w.taskAttemptForTask(lease); err == nil && ok && attempt.State == TaskStalled {
			return "act_stalled", nil
		}
	}
	r, e := w.LoadResult()
	if e == nil && r.JobID == jobID {
		if leaseErr == nil && lease.JobID == jobID && !resultMatchesTask(lease, r) {
			return "unconfirmed", nil
		}
		if taskTerminal(r.Status) {
			return "act_terminal", &r
		}
		if r.Status == "started" || r.Status == "in_progress" {
			return "act_running", nil
		}
	}
	if x, e := readJSON[DispatchReceipt](w.path("wb_dispatch_" + jobID + ".json")); e == nil && x.JobID == jobID &&
		(leaseErr != nil || ((x.AttemptID == "" || x.AttemptID == normalizeTaskIdentity(lease).AttemptID) && (x.LogicalJobID == "" || x.LogicalJobID == normalizeTaskIdentity(lease).LogicalJobID))) {
		return "wb_verified", nil
	}
	return "unconfirmed", nil
}

func formatObservation(s string) string {
	switch s {
	case "act_running":
		return "Meshing 已执行 ACT 任务"
	case "act_stalled":
		return "Meshing ACT 曾接收任务，但超过 90 秒没有新心跳（未确认停止）"
	case "act_terminal":
		return "Meshing 已返回终态结果或终态标记；仍以核验状态为准"
	case "wb_verified":
		return "Workbench 已定位 Mesh 容器；Meshing 尚未确认收到"
	default:
		return "尚无可核验的 Workbench/Meshing 阶段标记"
	}
}

func unknownDispatchError(id, observed string, elapsed time.Duration) error {
	return fmt.Errorf("任务 %s 已等待 %d 秒：%s。无法证明 Meshing 已停止，保留任务锁。请点击『检查任务状态』，不要重复提交", id, int(elapsed.Seconds()), formatObservation(observed))
}
