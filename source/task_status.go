package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type DispatchReceipt struct {
	JobID     string  `json:"job_id"`
	Status    string  `json:"status"`
	Stage     string  `json:"stage"`
	Timestamp float64 `json:"timestamp"`
}

func (w Workspace) taskStatus() string {
	lease, err := readJSON[Task](w.taskLeasePath())
	if err != nil {
		if os.IsNotExist(err) {
			if last, e := w.LoadResult(); e == nil && last.Stage == "recovered_from_job_bound_warning_plan" {
				return "当前没有待确认任务占用。\n上次任务：" + last.Phase + "（" + last.JobID + "）\n已从同一 Job ID 的局部方案文件恢复预览结果：" + last.Status + "\n未修改 Mesh，也没有重新投递。"
			}
			if last, e := w.LoadResult(); e == nil && last.Stage == "recovered_from_ACT_terminal_marker" {
				return "当前没有待确认任务占用。\n上次任务：" + last.Phase + "（" + last.JobID + "）\n已从 ACT 明确的失败/取消信号恢复终态：" + last.Status + "\n原因：" + last.Error + "\n没有重新投递，也没有操作网格。"
			}
			return "当前没有待确认的 AutoMesh 任务占用。"
		}
		return "任务占用记录不能读取：" + err.Error()
	}
	lines := []string{"任务 ID：" + lease.JobID, "操作：" + lease.Phase}
	if req, e := readJSON[TaskCancelRequest](w.cancelPath(lease.JobID)); e == nil && req.JobID == lease.JobID {
		lines = append(lines, "已请求安全停止："+req.RequestedAt+"；尚不代表 Meshing 已停止")
	}
	receipt, e := readJSON[DispatchReceipt](w.path("wb_dispatch_" + lease.JobID + ".json"))
	if e == nil && receipt.JobID == lease.JobID {
		lines = append(lines, "Workbench 路由："+receipt.Status+"（不代表 Meshing 已运行）")
	} else {
		lines = append(lines, "Workbench 路由：未获确认")
	}
	progress, e := readJSON[ActProgress](w.path("job_progress_" + lease.JobID + ".json"))
	if e == nil && progress.JobID == lease.JobID {
		lines = append(lines, "Meshing 阶段："+progress.Stage+"；"+progress.Detail)
		if progress.ProcessPID > 0 {
			lines = append(lines, fmt.Sprintf("ACT 宿主 PID：%d（读取自同一任务回执）", progress.ProcessPID))
			life, reason := inspectMeshingProcess(progress.ProcessPID, progress.ProcessStartedFiletime)
			lines = append(lines, "宿主进程状态："+life+"；"+reason)
		}
	} else {
		lines = append(lines, "Meshing 阶段：未获确认")
	}
	result, terminal := w.terminalResultForJob(lease.JobID)
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
	if lease, leaseErr := readJSON[Task](w.taskLeasePath()); leaseErr == nil && lease.JobID == jobID {
		if progress, progressErr := readJSON[ActProgress](w.path("job_progress_" + jobID + ".json")); progressErr == nil && progress.JobID == jobID &&
			(progress.Stage == "终态结果全部写入失败" || progress.Stage == "部分回执写入失败" || progress.Stage == "任务失败" || progress.Stage == "已安全取消") {
			_, _, _ = w.recoverTerminalProgress(lease)
		}
	}
	r, e := w.LoadResult()
	if terminal, ok := w.terminalResultForJob(jobID); ok {
		return "act_terminal", &terminal
	}
	if e == nil && r.JobID == jobID {
		if taskTerminal(r.Status) {
			return "act_terminal", &r
		}
		if r.Status == "started" || r.Status == "in_progress" {
			return "act_running", nil
		}
	}
	if p, e := readJSON[ActProgress](w.path("job_progress_" + jobID + ".json")); e == nil && p.JobID == jobID {
		if p.Timestamp > 0 && time.Since(time.Unix(int64(p.Timestamp), 0)) > 90*time.Second {
			return "act_stalled", nil // Lack of a heartbeat is not evidence that ACT exited.
		}
		return "act_running", nil
	}
	if x, e := readJSON[DispatchReceipt](w.path("wb_dispatch_" + jobID + ".json")); e == nil && x.JobID == jobID {
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
		return "Meshing 已写入结果"
	case "wb_verified":
		return "Workbench 已定位 Mesh 容器；Meshing 尚未确认收到"
	default:
		return "尚无可核验的 Workbench/Meshing 阶段标记"
	}
}

func unknownDispatchError(id, observed string, elapsed time.Duration) error {
	return fmt.Errorf("任务 %s 已等待 %d 秒：%s。无法证明 Meshing 已停止，保留任务锁。请点击『检查任务状态』，不要重复提交", id, int(elapsed.Seconds()), formatObservation(observed))
}
