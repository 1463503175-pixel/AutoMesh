package main

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed assets/meshing_worker.py assets/workbench_bridge.py assets/wb_cli.py assets/geometry_rules.py assets/warning_refinement.py assets/wb_server_bootstrap.py assets/start_wb_server_uia.ps1
var embedded embed.FS

type Body struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Volume    float64 `json:"volume"`
	FaceCount int     `json:"face_count"`
}

// Face retains optional read-only ACT geometry evidence for local defect review.
// Existing JSON snapshots without these fields remain compatible.
type Face struct {
	ID       int           `json:"id"`
	Body     int           `json:"body"`
	Area     float64       `json:"area"`
	Type     string        `json:"type"`
	Centroid []float64     `json:"centroid,omitempty"`
	BBox     []float64     `json:"bbox,omitempty"`
	Edges    []FeatureEdge `json:"feature_edges,omitempty"`
}

// FeatureEdge describes an actual ACT geometry edge, not a mesh element edge.
// Its ID is a reference for manual inspection, never an auto-publish scope.
type FeatureEdge struct {
	ID     int     `json:"id"`
	Length float64 `json:"length"`
}
type Named struct {
	Name string `json:"name"`
	IDs  []int  `json:"ids"`
}
type Snapshot struct {
	GeometryUnit                     string           `json:"geometry_unit"`
	DisplayUnitSystem                string           `json:"display_unit_system"`
	DisplayLengthUnit                string           `json:"display_length_unit"`
	Project                          string           `json:"project"`
	Fingerprint                      string           `json:"fingerprint"`
	GeometryQuickSignature           string           `json:"geometry_quick_signature,omitempty"`
	MeshSignature                    string           `json:"mesh_signature,omitempty"`
	Bodies                           []Body           `json:"bodies"`
	Faces                            []Face           `json:"faces"`
	Named                            []Named          `json:"named"`
	Controls                         []map[string]any `json:"controls"`
	ControlsComplete                 bool             `json:"controls_complete,omitempty"`
	WarningCoordinateSystems         []map[string]any `json:"warning_coordinate_systems,omitempty"`
	WarningCoordinateSystemsComplete bool             `json:"warning_coordinate_systems_complete,omitempty"`
	Mesh                             map[string]any   `json:"mesh"`
}
type Control struct {
	Name        string  `json:"name"`
	ControlName string  `json:"control_name,omitempty"`
	Category    string  `json:"category"`
	MM          float64 `json:"mm,omitempty"`
	IDs         []int   `json:"ids,omitempty"`
	Count       int     `json:"count,omitempty"`
	SkipReason  string  `json:"skip_reason,omitempty"`
}
type Plan struct {
	ExistingNamedSizes      map[string]float64 `json:"existing_named_sizes,omitempty"`
	ExistingNamedScopeIDs   map[string][]int   `json:"existing_named_scope_ids,omitempty"`
	ExpectedProject         string             `json:"expected_project"`
	Fingerprint             string             `json:"geometry_fingerprint"`
	GlobalMM                float64            `json:"global_mm"`
	MaxMM                   float64            `json:"max_mm"`
	ApplyGlobal             bool               `json:"apply_global"`
	LocalOverrides          map[string]float64 `json:"local_overrides,omitempty"`
	DisabledRegions         []string           `json:"disabled_regions,omitempty"`
	GroupOverrides          map[string]float64 `json:"group_overrides,omitempty"`
	DisabledGroups          []string           `json:"disabled_groups,omitempty"`
	EnabledGroups           []string           `json:"enabled_groups,omitempty"`
	ResumeOwned             bool               `json:"resume_owned_controls,omitempty"`
	AllowExistingMeshSizing bool               `json:"allow_existing_mesh_sizing,omitempty"`
	Controls                []Control          `json:"controls"`
	Review                  []Control          `json:"review"`
	SkewMin                 float64            `json:"-"`
	SkewMax                 float64            `json:"-"`
	QualityMin              float64            `json:"-"`
	QualityMax              float64            `json:"-"`
	Policy                  string             `json:"policy"`
}
type Task struct {
	// JobID remains the unique per-execution ID consumed by the existing ACT protocol.
	JobID                     string  `json:"job_id"`
	LogicalJobID              string  `json:"logical_job_id,omitempty"`
	AttemptID                 string  `json:"attempt_id,omitempty"`
	AttemptNumber             int     `json:"attempt_number,omitempty"`
	RetryOfJobID              string  `json:"retry_of_job_id,omitempty"`
	Phase                     string  `json:"phase"`
	MeshSystem                string  `json:"mesh_system,omitempty"`
	SubmittedAt               string  `json:"submitted_at,omitempty"`
	ProtocolVersion           int     `json:"protocol_version"`
	ExpectedProject           string  `json:"expected_project,omitempty"`
	Fingerprint               string  `json:"geometry_fingerprint,omitempty"`
	Plan                      *Plan   `json:"plan,omitempty"`
	ExportPath                string  `json:"export_path,omitempty"`
	Overwrite                 bool    `json:"overwrite,omitempty"`
	QualityReviewAcknowledged bool    `json:"quality_review_acknowledged,omitempty"`
	DeepNativeSkewness        bool    `json:"deep_native_skewness,omitempty"`
	AllowExistingMeshSizing   bool    `json:"allow_existing_mesh_sizing,omitempty"`
	BaseMM                    float64 `json:"base_mm,omitempty"`
	SkewMin                   float64 `json:"skew_min"`
	SkewMax                   float64 `json:"skew_max"`
	QualityMin                float64 `json:"quality_min,omitempty"`
	QualityMax                float64 `json:"quality_max,omitempty"`
	ApplyGlobal               bool    `json:"apply_global,omitempty"`
}
type Result struct {
	JobID              string          `json:"job_id"`
	LogicalJobID       string          `json:"logical_job_id,omitempty"`
	AttemptID          string          `json:"attempt_id,omitempty"`
	AttemptNumber      int             `json:"attempt_number,omitempty"`
	RetryOfJobID       string          `json:"retry_of_job_id,omitempty"`
	VerificationStatus string          `json:"verification_status,omitempty"`
	Phase              string          `json:"phase"`
	Status             string          `json:"status"`
	Stage              string          `json:"stage,omitempty"`
	CompletedAt        string          `json:"completed_at,omitempty"`
	CompletedAtUnix    float64         `json:"completed_at_unix,omitempty"`
	Summary            json.RawMessage `json:"summary"`
	Error              string          `json:"error"`
}

func friendlyACTError(trace string) string {
	if strings.Contains(trace, "审核生成前无法确认 Meshing 全局网格尺寸") {
		return "当前 Meshing 全局 Element Size 显示为 0 / 自动或无法验证。局部 Sizing 可以单独发布，但出于 32 GB 内存保护，尚不能清除旧网格并重新生成。请先在 Meshing 中确认并设置明确的全局尺寸（mm），再执行审核生成。"
	}
	if strings.Contains(trace, "32GB surface refinement budget exceeded") {
		return "当前选中命名组的局部细化预算超出 32 GB 保守阈值（这是面积/尺寸²代理值，不是实际单元数）。请减少启用的组，或调大局部尺寸后重试；已有网格和命名不会被清除。"
	}
	if strings.Contains(trace, "Selected Named Selection scopes overlap") {
		return "选中的命名组包含重叠面。请在 Meshing 检查重复作用域，取消重叠分组后重新发布。"
	}
	if strings.Contains(trace, "Selected Named Selection no longer exists:") {
		tail := trace[strings.LastIndex(trace, "Selected Named Selection no longer exists:")+len("Selected Named Selection no longer exists:"):]
		name := strings.TrimSpace(strings.SplitN(tail, ";", 2)[0])
		if len([]rune(name)) > 90 {
			name = string([]rune(name)[:90]) + "…"
		}
		return "当前 Meshing 找不到命名组「" + name + "」。软件未修改网格或命名；请先刷新命名。如组被中文化，程序仅会在面范围完全一致且匹配唯一时自动关联新名称。"
	}
	if strings.Contains(trace, "no valid geometric face scope") {
		return "当前 Named Selection 面范围为空或存在过期面 ID。请到 Meshing 检查该组作用面并重新只读扫描。"
	}
	if strings.Contains(trace, "More than one existing Sizing is bound") {
		return "该命名组已绑定不止一个 Sizing。为防止重复细化，软件拒绝叠加控制；请在 Meshing 中检查现有 Sizing。"
	}
	if strings.Contains(trace, "No Named Selection selected") {
		return "请先在局部细化页选择命名组、勾选启用、设置尺寸并点击「保存本组尺寸」。"
	}
	if strings.Contains(trace, "explicit Sizing publication consent required") {
		return "当前工程已有网格。请先使用「仅发布命名」；若要发布 Sizing，请先只读扫描并明确同意网格可能失效的风险。"
	}
	if strings.Contains(trace, "global size overwrite prohibited") {
		return "已有网格时不允许覆盖全局尺寸；请关闭「覆盖全局尺寸」后重新扫描。"
	}
	if strings.Contains(trace, "Generated mesh exists") {
		return "检测到已有网格：不会自动重划或覆盖现有网格。请在 Meshing 检查当前状态。"
	}
	if strings.Contains(trace, "Named selections changed after inspect") {
		return "命名组已变化，请重新执行「只读扫描」，再进行下一步。"
	}
	lines := strings.Split(strings.TrimSpace(trace), "\n")
	if len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last != "" {
			r := []rune(last)
			if len(r) > 280 {
				return string(r[:280])
			}
			return last
		}
	}
	return "Meshing 返回未知错误；完整技术诊断已保存在任务结果中。"
}

type Workspace struct{ Dir string }

// ACT/IronPython execution is sensitive to non-ASCII paths in some CAD toolchains.
// Keep all generated scripts and task files outside the user's localized home path.
func asciiPath(path string) bool {
	if path == "" {
		return false
	}
	for _, ch := range path {
		if ch > 127 {
			return false
		}
	}
	return true
}

func DefaultWorkspace() (Workspace, error) {
	if env := os.Getenv("AUTOMESH_DATA_DIR"); env != "" {
		if err := validateDataDirectory(env); err != nil {
			return Workspace{}, err
		}
		if err := os.MkdirAll(env, 0755); err != nil {
			return Workspace{}, err
		}
		return Workspace{Dir: filepath.Clean(env)}, nil
	}
	configured, found, configErr := loadConfiguredDataDirectory()
	if configErr != nil {
		return Workspace{}, configErr
	}
	if found {
		if err := os.MkdirAll(configured, 0755); err != nil {
			return Workspace{}, fmt.Errorf("configured workspace unavailable: %w", err)
		}
		probe := filepath.Join(configured, ".write_probe")
		if err := os.WriteFile(probe, []byte("ok"), 0600); err != nil {
			return Workspace{}, err
		}
		_ = os.Remove(probe)
		return Workspace{Dir: configured}, nil
	}
	candidates := []string{}
	if x := os.Getenv("ProgramData"); x != "" {
		candidates = append(candidates, filepath.Join(x, "AutoMesh2024R2", "Data"))
	}
	if x := os.Getenv("PUBLIC"); x != "" {
		candidates = append(candidates, filepath.Join(x, "Documents", "AutoMesh2024R2", "Data"))
	}
	for _, path := range candidates {
		if !asciiPath(path) {
			continue
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			continue
		}
		probe := filepath.Join(path, ".write_probe")
		if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
			continue
		}
		_ = os.Remove(probe)
		return Workspace{Dir: path}, nil
	}
	return Workspace{}, errors.New("未找到可写的英文路径工作目录；请检查 ProgramData / PUBLIC 目录权限")
}

// No second dispatch if the remote call returns before the asynchronous ACT worker.
// A complete result must match this job ID and have an explicit terminal status.
// Recover only completed leases. An abandoned/unknown job must be inspected
// in the real Meshing session before anybody removes its lease manually.
func (w Workspace) taskLeasePath() string { return w.path("active_task.json") }

// A cancellation request is a DURABLE, cooperative signal. It never kills the
// ANSYS process or proves that a native GenerateMesh call has stopped.
type TaskCancelRequest struct {
	JobID       string `json:"job_id"`
	RequestedAt string `json:"requested_at"`
	Reason      string `json:"reason"`
}

func (w Workspace) cancelPath(jobID string) string { return w.path("cancel_" + jobID + ".json") }
func (w Workspace) RequestTaskCancel(jobID, reason string) error {
	if len(jobID) != 32 {
		return errors.New("无效的任务 ID")
	}
	lease, err := readJSON[Task](w.taskLeasePath())
	if err != nil || lease.JobID != jobID {
		return errors.New("目标任务不再持有当前锁；拒绝发送取消请求")
	}
	return writeJSON(w.cancelPath(jobID), TaskCancelRequest{JobID: jobID,
		RequestedAt: time.Now().Format(time.RFC3339Nano), Reason: reason})
}
func taskTerminal(status string) bool {
	return status == "ok" || status == "error" || status == "cancelled"
}
func (w Workspace) acquireTaskLease(t Task) error {
	path := w.taskLeasePath()
	for n := 0; n < 2; n++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			b, marshalErr := json.MarshalIndent(t, "", "  ")
			if marshalErr == nil {
				_, marshalErr = f.Write(append(b, '\n'))
			}
			if syncErr := f.Sync(); marshalErr == nil {
				marshalErr = syncErr
			}
			closeErr := f.Close()
			if marshalErr != nil || closeErr != nil {
				_ = os.Remove(path)
				return fmt.Errorf("无法保存任务占用记录: %v %v", marshalErr, closeErr)
			}
			return nil
		}
		if !os.IsExist(err) {
			return fmt.Errorf("无法创建排他任务锁: %w", err)
		}
		old, readErr := readJSON[Task](path)
		if readErr != nil || old.JobID == "" {
			return errors.New("存在不可读取的 active_task.json；可能有另一任务运行，禁止覆盖")
		}
		terminal, finished := w.terminalResultForTask(old)
		if finished {
			// Persist the terminal lifecycle state before freeing a verified lease.
			if e := w.recordTaskResult(old, terminal); e != nil {
				return fmt.Errorf("任务已终止，但无法保存历史状态；保留任务锁：%w", e)
			}
			// No unattended stale-timeout deletion.
			if e := os.Remove(path); e != nil {
				return fmt.Errorf("任务已结束但无法解锁: %w", e)
			}
			continue
		}
		return fmt.Errorf("存在未确认结束的 Meshing 任务 %s（%s）；请先确认原 Meshing 已停止。不会覆盖 job.json 或自动重新提交", old.JobID, old.Phase)
	}
	return errors.New("排他任务锁获取失败；检查任务日志")
}

// Explicit operator-controlled recovery for a stalled job with no terminal
// result. Never runs automatically: the application cannot prove that a
// hidden/remote Meshing worker has stopped. Keep an audit copy of the old task.
func (w Workspace) AbandonTaskLease(approval string) (string, error) {
	if approval != "I_CONFIRMED_MESHING_STOPPED" {
		return "", errors.New("解除任务锁需要明确人工确认")
	}
	old, err := readJSON[Task](w.taskLeasePath())
	if err != nil {
		return "", err
	}
	if len(old.JobID) != 32 {
		return "", errors.New("任务锁记录 ID 无效，拒绝解除")
	}
	if _, err := hex.DecodeString(old.JobID); err != nil {
		return "", errors.New("任务锁记录 ID 无效")
	}
	last, e := w.LoadResult()
	info := map[string]any{"task": old, "abandoned_at": time.Now().Format(time.RFC3339),
		"reason": "user explicitly confirmed the previous Meshing job is stopped"}
	if e == nil {
		info["observed_last_result"] = last
	}
	audit := w.path("abandoned_" + old.JobID + ".json")
	if err := writeJSON(audit, info); err != nil {
		return "", fmt.Errorf("无法保存旧任务审计记录，拒绝解除: %w", err)
	}
	// Check lease owner again before changing anything.
	current, err := readJSON[Task](w.taskLeasePath())
	if err != nil || current.JobID != old.JobID {
		return "", errors.New("任务占用记录发生变化，拒绝解除")
	}
	if err := os.Remove(w.taskLeasePath()); err != nil {
		return "", err
	}
	return audit, nil
}

// Never bootstrap a new remote service or accept a second modifying task while
// the last ACT job's completion is unconfirmed. A process restart is not proof.
func (w Workspace) UnconfirmedActiveTask() (Task, bool, error) {
	t, err := readJSON[Task](w.taskLeasePath())
	if os.IsNotExist(err) {
		return Task{}, false, nil
	}
	if err != nil {
		return Task{}, true, err
	}
	if t.JobID == "" {
		return t, true, errors.New("当前任务记录缺少 ID")
	}
	if result, terminal := w.terminalResultForTask(t); terminal {
		if err := w.recordTaskResult(t, result); err != nil {
			return t, true, err
		}
		return t, false, nil
	}
	return t, true, nil
}
func (w Workspace) ReconcileFinishedLease() error {
	lease, err := readJSON[Task](w.taskLeasePath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("无法读取旧任务占用: %w", err)
	}
	if _, found := w.terminalResultForTask(lease); found {
		return w.releaseTaskLease(lease.JobID)
	}
	// ACT may have emitted its terminal failure marker but failed to save
	// the last_result.json file. Safely reconstruct its error receipt.
	if _, recovered, e := w.recoverTerminalProgress(lease); e != nil {
		return e
	} else if recovered {
		return w.releaseTaskLease(lease.JobID)
	}
	// The old ACT job might still be running. Allow the initializer to decide
	// whether a script update is needed; NEVER remove this record by timeout.
	return nil
}
func (w Workspace) releaseTaskLease(jobID string) error {
	old, err := readJSON[Task](w.taskLeasePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if old.JobID != jobID {
		return errors.New("任务 ID 不匹配，拒绝解除占用")
	}
	last, terminal := w.terminalResultForTask(old)
	if !terminal {
		return errors.New("仅可在 Meshing 结果确认为终态时解除任务锁")
	}
	if err := w.recordTaskResult(old, last); err != nil {
		return fmt.Errorf("任务终态已收到，但历史状态无法持久化；保留任务锁：%w", err)
	}
	// If the per-job receipt survived but last_result.json did not, repair
	// the UI's current report before dropping the lease.
	generic, e := w.LoadResult()
	if e != nil || !resultMatchesTask(old, generic) || !taskTerminal(generic.Status) {
		if e := writeJSON(w.path("last_result.json"), last); e != nil {
			return e
		}
	}
	return os.Remove(w.taskLeasePath())
}

// ACT reports per-job heartbeats separately from its terminal result.
// A nil percentage means ANSYS has no verifiable progress ratio (e.g. GenerateMesh).
type ActProgress struct {
	JobID                  string  `json:"job_id"`
	LogicalJobID           string  `json:"logical_job_id,omitempty"`
	AttemptID              string  `json:"attempt_id,omitempty"`
	AttemptNumber          int     `json:"attempt_number,omitempty"`
	RetryOfJobID           string  `json:"retry_of_job_id,omitempty"`
	Stage                  string  `json:"stage"`
	Detail                 string  `json:"detail"`
	Percent                *int    `json:"percent"`
	ProcessPID             int     `json:"process_pid,omitempty"`
	ProcessStartedFiletime string  `json:"process_started_filetime,omitempty"`
	Timestamp              float64 `json:"timestamp,omitempty"`
}

func displayActProgress(p ActProgress) string {
	if p.Percent == nil {
		return "I|" + p.Stage + "|" + p.Detail
	}
	n := *p.Percent
	if n < 0 || n > 100 {
		return "I|" + p.Stage + "|" + p.Detail
	}
	return fmt.Sprintf("P%d|%s|%s", n, p.Stage, p.Detail)
}

func (w Workspace) WaitResult(jobID string, timeout, interval time.Duration, progress func(string)) (Result, error) {
	if len(jobID) != 32 {
		return Result{}, errors.New("无效任务 ID")
	}
	if interval <= 0 {
		interval = time.Second
	}
	if timeout <= 0 {
		timeout = 45 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	noticeDeadline := time.Now().Add(75 * time.Second)
	reportedUnconfirmed := false
	seenCurrentJob := false
	previousStage := ""
	lastProgress := ""
	lastHeartbeat := time.Time{}
	begin := time.Now()
	for {
		if p, err := readJSON[ActProgress](w.path("job_progress_" + jobID + ".json")); err == nil && p.JobID == jobID {
			formatted := displayActProgress(p)
			if formatted != lastProgress {
				if progress != nil {
					progress(formatted)
				}
				lastProgress = formatted
				lastHeartbeat = time.Now()
			}
		}
		var terminal Result
		var found bool
		if lease, leaseErr := readJSON[Task](w.taskLeasePath()); leaseErr == nil && lease.JobID == jobID {
			terminal, found = w.terminalResultForTask(lease)
		} else {
			terminal, found = w.terminalResultForJob(jobID)
		}
		if found {
			if lease, leaseErr := readJSON[Task](w.taskLeasePath()); leaseErr == nil && lease.JobID == jobID {
				_ = w.recordTaskResult(lease, terminal)
			}
			if unlockErr := w.releaseTaskLease(jobID); unlockErr != nil && progress != nil {
				progress("任务已结束，但无法释放任务占用记录：" + unlockErr.Error())
			}
			return terminal, nil
		}
		// The terminal ACT marker is emitted after the operation's exception
		// handler. If final JSON was lost, recover this job's error (never a
		// successful result) instead of waiting for 45 minutes.
		if lease, le := readJSON[Task](w.taskLeasePath()); le == nil && lease.JobID == jobID {
			if terminal, found, recoveryErr := w.recoverTerminalProgress(lease); recoveryErr == nil && found {
				if unlockErr := w.releaseTaskLease(jobID); unlockErr != nil && progress != nil {
					progress("任务已结束，但无法释放任务占用记录：" + unlockErr.Error())
				}
				return terminal, nil
			}
		}
		r, err := w.LoadResult()
		if err == nil && r.JobID == jobID {
			if lease, leaseErr := readJSON[Task](w.taskLeasePath()); leaseErr == nil && lease.JobID == jobID && !resultMatchesTask(lease, r) {
				r = Result{}
			} else {
				seenCurrentJob = true
			}
			switch r.Status {
			case "ok", "error", "cancelled":
				if unlockErr := w.releaseTaskLease(jobID); unlockErr != nil && progress != nil {
					progress("任务已结束，但无法释放任务占用记录：" + unlockErr.Error())
				}
				return r, nil
			case "in_progress", "started":
				stage := r.Stage
				if stage == "" {
					stage = r.Status
				}
				if stage != previousStage {
					if progress != nil && lastProgress == "" {
						progress("I|ANSYS 正在执行|" + stage + "；请勿重复提交")
					}
					previousStage = stage
				}
			}
		}
		if reason, crashed := w.meshingProcessCrash(jobID); crashed {
			if progress != nil {
				progress("I|Meshing 进程已退出|" + reason)
			}
			return Result{}, fmt.Errorf("%s。保留任务锁和历史结果，禁止自动重发", reason)
		}
		// Heartbeat: elapsed time is real, but NEVER turn it into a fake %.
		if progress != nil && (lastHeartbeat.IsZero() || time.Since(lastHeartbeat) >= 6*time.Second) {
			label := "正在等待 Meshing 执行，未收到可用阶段数据"
			if lastProgress != "" {
				label = "尚未收到新阶段数据；Meshing 可能仍在计算"
			}
			progress(fmt.Sprintf("I|等待任务状态|%s（已运行 %d 秒）", label, int(time.Since(begin).Seconds())))
			lastHeartbeat = time.Now()
		}
		if !seenCurrentJob && !reportedUnconfirmed && !time.Now().Before(noticeDeadline) {
			if progress != nil {
				progress("I|远端阶段暂未确认|Workbench 可能仍在排队；不解除锁、不自动重投递。可查看任务状态")
			}
			reportedUnconfirmed = true
		}
		if !time.Now().Before(deadline) {
			return Result{}, fmt.Errorf("等待任务 %s 的 ANSYS 结果超时；任务可能仍在 Meshing 中运行，禁止重复提交", jobID)
		}
		time.Sleep(interval)
	}
}

func (w Workspace) path(name string) string { return filepath.Join(w.Dir, name) }
func readJSON[T any](filename string) (T, error) {
	var out T
	b, err := os.ReadFile(filename)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(b, &out)
	return out, err
}
func writeJSON(filename string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeAtomic(filename, b)
}
func writeAtomic(filename string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	temp := filename + ".tmp"
	if err := os.WriteFile(temp, b, 0644); err != nil {
		return err
	}
	if err := os.Rename(temp, filename); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}

// Only replace embedded generated worker files from verified AutoMesh releases.
// Unknown or user-edited scripts must be examined rather than silently lost.
var knownLegacyWorkerHashes = map[string]bool{
	"b3ae587045e191ea2eeb45a20b02434c2e146497b1a4f8c84a81adfc6db5dc9b": true, // verified R8.1 RC 2026-11-08 / 2026-11-09 worker
	"db33a9c1641a741249010435515937a6828734eede6c7622eb9e7c967f3e4ac1": true, // verified R8.1 RC EQ worker
	"c35a1f010ecdcf26b1b343355a8f8769e84b9afff1dfa916d42be08cb5a4ecb2": true, // exact stage 2E verified worker, safe upgrade to opt-in quality distribution
	"c6bbc68afb28ac07add565021498544c29a4bcc4928d6bc8527f9bc866a991d6": true, // exact R8.0.3 worker; permits safe R8.0.4 continuous-refinement upgrade
	"5a9f10af7ad53b9ff168f52151375bfd87a456ec3c09b2627eb37dde745153c7": true, // exact R8.0.1 worker; permits safe in-place R8.0.2 upgrade
	"7d4ec6d73f06a3c701535d71765f903b4d29be7a59d5b45bbf0bb66a49ca25e4": true, // verified R7.9 original release Worker for safe R8.0 upgrade
	"a6d252a4d0a57c6b723b494280922c0cf83a0c3743e749bb4f6f4345037820d9": true, // verified R7.5 release meshing_worker.py; R7.6 safe upgrade
	"1e5af1689430ac9c4c10ff63bee7065ca808b9bbe447251dfb37f6136e6b7b04": true, // verified R7.4 distribution, required for safe R7.4 -> R7.4.1 upgrade
	"56541390300fdd1c1daabba0d5070fbf6ab3c438f4b1d6e75601b883e67c6210": true, // verified R7.2/R7.3 official source before range selection
	"ec5df411917e86bf1d705042d27127ba1e8355d0a361c59afe8971e24a209bf7": true, // verified R7.2/R7.3 official source before range selection
	"a852ad605154c97a24d5f28a785dd4a5acd0646c55a5cb17eb211a88dae126f3": true, // R7 official release worker; permits R7 -> R7.1 after safe terminal reconciliation
	"dd757e364d92d0c39045023356515fe4b8ecbf4291e2ba20caf7a9e72db8b379": true, // R6.4 working source, pre-R7 verified bytes
	"0be57e7b331730cd06f41de933486b34f89888f10689f537df1c337fd41e7ad1": true, // R6.3.2 AutoStart official worker
	"61f03fba8e5fdeedba965c351d72b68906507456b45e4a73dab58bf8d9b83582": true, // R6.2 official release, CrashWatch upgrade
	"d039bd4a5d70ea9ced93ff03a0652ba67f1b25f006295c984dfb547e30e3d35f": true, // R6/R6.1 bundled worker
	"8188e0608d378da97d9d1e0876cf79108fe9dce11dc8ceb14fa2ee83":         true, // R5.2/R5.3 tested release

	"713a6e63770b73a5836ea76955cc19806e7e5da8ff7293b088ccce4da18270d0": true, // R5.1 real-world Global Size Fix
	"0645a99ebac23328e9d5588bac3b82b8f1e7bf4a3b715e6e277fe2d1887d5c5e": true, // R5 Model Readiness Fix
	"b249ffa220dc4ecf43048e94b5d6ab417b2c5e8f0980a67536ffd8a8edfcbecc": true, // R4 integrated release, safe R5 in-place upgrade
	"3cda90d5fa8935340e0b1042e68c39970797e1ce682076d870291d453fec5f4b": true, // R3 integrated source
	"5d27be9d6384939763108bb2472c9e35c81a408a0de1ff3969d65ac1bf778426": true, // source v0.10.8
	"9a13d6e76c4f06f38a4628c46e72d6800c6893acc116196c0f1edbe2191dc5fc": true, // Location fix
	"e20426460439bbf06002f3d13bc0a099934c9a39f4386bf05ed012ac3e9bbd68": true, // Republish fix
	"3b4aca592b51d8a36da9e12a5f0cde72e80fbb5ad63fd99b52ad63970990574f": true, // Quality/Remesh
	"5f0534e9e7c0b94da50a6d470ac946d1ccebbec504ee4e51b56697e7b100ee77": true, // Reviewed R2
}

// Permit only verified untouched bridge scripts from earlier releases.
// Everything else stays protected, including user-modified connection scripts.
var knownLegacyAssetHashes = map[string]map[string]bool{
	"start_wb_server_uia.ps1": {
		"2938671afa2a15cb901db55b0c4b0f8e848d6ffb7346e4a720b8ad6a50871d30": true, // verified official R7.2/R7.3 source
	},
	// Current R7.1 assets must be eligible for one atomic in-place upgrade.
	// Unknown/user-edited scripts remain protected.
	"wb_cli.py": {
		"124845e25cca6520bb1b8e6d6c4f482099ef0a72dc5221fd6b8dc1254813f204": true, // verified official R7.2/R7.3 source
		"84433352e7075c56676207f6fb7f5606db56ccdbfee071e789421e653b0adf10": true,
	},
	"wb_server_bootstrap.py": {
		"0426f112a80836dc53b288776bef5f68f49c6e3fbc81a1a68942edbdebbcd09e": true, // verified official R7.2/R7.3 source
		"81425fe68cac883d714997f38df8768ee34517b10322cd2ec7baa9811fd7f169": true, // verified official R7.2/R7.3 source
		"5f39429a1afb9267e5870cbe87dd73c5048ea1d3d4081c4bafb9b7707b9f5acc": true,
	},
	"warning_refinement.py": {
		"2e2c6bff9553e78b2ac641c285e62c7c5bbf4230bcf835ab4c3996158ba6c86a": true, // verified R8.1 2026-11-08 RC module
		"c123df29ed187691d5701d8c6d298c6d8f18ce7b09c1e6042a92822caa2641ea": true, // verified R8.1 EQ interval module
		"8c8a393f8dbd30828c7007d544476b5cf5d16ac8a81a73f6d4a8216d08d0ec28": true, // verified R8.1 EQ budget module
		"899fbef06a7285b03168ef935fde9394ce9c2d605180baa8920d59b9641711fd": true, // exact R8.0.3 warning module; permits safe R8.0.4 continuous-refinement upgrade
		"eaebbad9c9dac0b1a9e18d755ef19d0b236cfe8b8db444e4b46e2b62ffd6139a": true, // exact R8.0.1 warning module; permits safe in-place R8.0.2 upgrade
		"c3c6e20c5a80eca010011630e07c9c2581dd2d0d254d7cd787871a7bfa00c39c": true, // verified R7.9 original release warning module for safe R8.0 upgrade
		"c13a583019f626528577642f00a7d15cc44ff5106604255d82affa811e9a2fc5": true, // verified R7.5 release warning_refinement.py; R7.6 safe upgrade
		"2ddb5539dc492ae2ffc3f8348c2f0a0b1c917a7e4d78651fa411f59e87c76162": true, // R7.4.1 original verified asset: safe R7.5 upgrade
		"e26cfcc334fe85394ea8e8a4bdef5fcfa48a89e2069acc410dbc922be33734b8": true, // verified official R7.2/R7.3 source
		"f4b595468c5040f181a8507d98d06aaa539f29541f1dbbd92c611ef6e46d47ab": true, // verified official R7.2/R7.3 source
		"38650c114a66c53e5ceeebe9c15c5cc305d2f092bd393a0b9f8bc34e9f01bb34": true, // R6.3.2 release
		"84dc9f5dbbc9f40728655ee2908ae803f2b364422a065cb15c3bb2fe1200dec3": true, // R6.4 working source
	},
	"workbench_bridge.py": {
		"3245b6c09f46a670064747d119f3d158d8a9262bc9525361ec2b864730e5abc3": true, // verified official R7.2/R7.3 source
		"a554ca6e688ba5f3a85ab7108d9e7a5a6f393890d27bd2fc04ac947fa03c66ef": true, // R5.1–R5.3
	},
}

func checkGeneratedAssetReplacement(name string, existing, embedded []byte) error {
	if sha256.Sum256(existing) == sha256.Sum256(embedded) {
		return nil
	}
	digest := sha256.Sum256(existing)
	hash := hex.EncodeToString(digest[:])
	if name == "meshing_worker.py" && knownLegacyWorkerHashes[hash] {
		return nil
	}
	if hashes, ok := knownLegacyAssetHashes[name]; ok && hashes[hash] {
		return nil
	}
	return fmt.Errorf("受保护脚本 %s 的版本无法识别，已保留文件；请勿删除 Data，提供 startup_diagnostic.json 核查（refusing to overwrite unrecognized/customized script, SHA256 %s）", name, hash)
}

type assetUpdate struct {
	Name    string
	Path    string
	New     []byte
	Old     []byte
	Existed bool
}

// Two-phase update: validate ALL files before replacing ANY script, back up
// originals by content hash, then commit. Never silently mix old worker with
// new bridge if validation of a later file fails.
func (w Workspace) InstallAssets() error {
	if err := os.MkdirAll(w.Dir, 0755); err != nil {
		return err
	}
	// Trust only a receipt written after a successful AutoMesh install.
	// Prior versions had no receipt: exact verified release hashes remain accepted.
	receipt := w.readInstalledAssetReceipt()
	names := managedScriptNames
	pending := []assetUpdate{}
	for _, name := range names {
		b, err := embedded.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		path := w.path(name)
		prior, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		existed := err == nil
		if existed && sha256.Sum256(b) == sha256.Sum256(prior) {
			continue
		}
		if existed {
			if err = checkGeneratedAssetReplacement(name, prior, b); err != nil && !receipt.owns(name, prior) {
				return err
			}
		}
		pending = append(pending, assetUpdate{Name: name, Path: path, New: b, Old: prior, Existed: existed})
	}
	if len(pending) > 0 {
		if _, err := os.Stat(w.taskLeasePath()); err == nil {
			oldTask, e := readJSON[Task](w.taskLeasePath())
			if e == nil && oldTask.JobID != "" {
				// The exact ACT terminal result is sufficient proof for the
				// previous lease, even if the UI crashed before cleanup.
				if _, finished := w.terminalResultForJob(oldTask.JobID); finished {
					if releaseErr := w.releaseTaskLease(oldTask.JobID); releaseErr != nil {
						return releaseErr
					}
				} else {
					return errors.New("已有未确认结束的 AutoMesh 任务；禁止更新 Worker/Workbench 脚本。先检查任务状态，不要重投递")
				}
			} else {
				return errors.New("任务锁不可读取，禁止升级任何脚本")
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// Stage ALL recoverable originals before any replacement. Preserve
	// snapshots for subsequent upgrades and never overwrite user data.
	for _, item := range pending {
		if !item.Existed {
			continue
		}
		digest := sha256.Sum256(item.Old)
		backup := item.Path + ".before-" + hex.EncodeToString(digest[:8]) + ".bak"
		if old, err := os.ReadFile(backup); err == nil {
			if sha256.Sum256(old) != digest {
				return fmt.Errorf("升级备份校验失败：%s", backup)
			}
		} else if os.IsNotExist(err) {
			if err := writeAtomic(backup, item.Old); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	written := []assetUpdate{}
	rollback := func(cause error) error {
		failures := []string{}
		for i := len(written) - 1; i >= 0; i-- {
			item := written[i]
			live, e := os.ReadFile(item.Path)
			if e != nil || sha256.Sum256(live) != sha256.Sum256(item.New) {
				failures = append(failures, item.Name+": 内容已变化，拒绝强行回滚")
				continue
			}
			if item.Existed {
				e = writeAtomic(item.Path, item.Old)
			} else {
				e = os.Remove(item.Path)
			}
			if e != nil {
				failures = append(failures, item.Name+": "+e.Error())
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("%w；回滚未完成：%s", cause, strings.Join(failures, "; "))
		}
		return cause
	}
	for _, item := range pending {
		if err := writeAtomic(item.Path, item.New); err != nil {
			return rollback(err)
		}
		written = append(written, item)
	}
	if err := w.WriteRunner(); err != nil {
		return rollback(err)
	}
	// Commit the next-version upgrade receipt only after all seven official
	// scripts and runner were successfully installed. It contains no task data.
	if err := w.writeInstalledAssetReceipt(); err != nil {
		return rollback(err)
	}
	return nil
}

func pyLiteral(p string) string {
	p = filepath.ToSlash(p)
	p = strings.ReplaceAll(p, "'", "\\'")
	return "u'" + p + "'"
}
func (w Workspace) WriteRunner() error {
	txt := "# -*- coding: utf-8 -*-\n# Only in current open ANSYS Meshing scripting console.\n" +
		"JOB_FILE = " + pyLiteral(w.path("job.json")) + "\n" +
		"RESULT_FILE = " + pyLiteral(w.path("last_result.json")) + "\n" +
		"SNAPSHOT_FILE = " + pyLiteral(w.path("snapshot.json")) + "\n" +
		"execfile(" + pyLiteral(w.path("meshing_worker.py")) + ")\n"
	out := w.path("Run_In_Existing_Meshing.py")
	if previous, err := os.ReadFile(out); err == nil && string(previous) == txt {
		return nil
	}
	return writeAtomic(out, []byte(txt))
}
func (w Workspace) Command() string {
	return "execfile(" + pyLiteral(w.path("Run_In_Existing_Meshing.py")) + ")"
}
func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (w Workspace) Submit(phase string, s *Snapshot, p *Plan, exportPath string, overwrite bool, qualityAck ...bool) (Task, error) {
	return w.submitForSystem("", phase, s, p, exportPath, overwrite, qualityAck...)
}

// SubmitForSystem binds the durable task and ACT job payload to the exact Mesh
// system selected in the existing Workbench session. The name is metadata; it
// does not connect, launch, or switch any ANSYS session.
func (w Workspace) SubmitForSystem(system, phase string, s *Snapshot, p *Plan, exportPath string, overwrite bool, qualityAck ...bool) (Task, error) {
	return w.submitForSystem(system, phase, s, p, exportPath, overwrite, qualityAck...)
}

func (w Workspace) submitForSystem(system, phase string, s *Snapshot, p *Plan, exportPath string, overwrite bool, qualityAck ...bool) (Task, error) {
	// An explicit deep read shares the existing read-only quality protocol.
	// Its opt-in flag never propagates to ordinary quality checks.
	deepNative := phase == "quality_check_deep"
	if deepNative {
		phase = "quality_check"
	}
	t := Task{JobID: newID(), LogicalJobID: newID(), AttemptID: newID(), AttemptNumber: 1,
		Phase: phase, MeshSystem: strings.TrimSpace(system), SubmittedAt: time.Now().Format(time.RFC3339Nano), ProtocolVersion: 2, DeepNativeSkewness: deepNative}
	if phase == "export" && len(qualityAck) > 0 && qualityAck[0] {
		t.QualityReviewAcknowledged = true
	}
	valid := map[string]bool{"inspect": true, "sync_status": true, "apply": true, "generate": true, "quality": true, "export": true, "auto": true, "auto_onepass": true, "prepare": true, "publish_names": true, "publish_existing_sizing": true, "generate_reviewed": true, "diagnose": true, "mark_failed": true, "mark_warning": true, "mark_high_skew": true, "mark_low_element_quality": true, "quality_check": true, "rename_auxiliary": true, "warning_refine_preview": true, "warning_refine_publish": true}
	if !valid[phase] {
		return t, errors.New("不支持该任务阶段")
	}
	if phase == "mark_high_skew" {
		if p == nil || p.SkewMin < 0 || p.SkewMax > 1 || p.SkewMin >= p.SkewMax ||
			(p.SkewMin != p.SkewMin) || (p.SkewMax != p.SkewMax) {
			return t, errors.New("偏度范围必须满足 0 <= 下限 < 上限 <= 1")
		}
		t.SkewMin, t.SkewMax = p.SkewMin, p.SkewMax
	}
	if phase == "mark_low_element_quality" {
		if p == nil || p.QualityMin < 0 || p.QualityMax > 1 || p.QualityMin >= p.QualityMax ||
			math.IsNaN(p.QualityMin) || math.IsNaN(p.QualityMax) || math.IsInf(p.QualityMin, 0) || math.IsInf(p.QualityMax, 0) {
			return t, errors.New("Element Quality 范围必须满足 0 <= 下限 < 上限 <= 1")
		}
		t.QualityMin, t.QualityMax = p.QualityMin, p.QualityMax
	}
	if phase == "inspect" {
		t.BaseMM = 12.0
		if p != nil && p.GlobalMM >= 0.5 && p.GlobalMM <= 500 {
			t.BaseMM = p.GlobalMM
		}
	}
	if phase != "inspect" && phase != "auto_onepass" && phase != "sync_status" &&
		!((phase == "quality_check" || phase == "diagnose") && s == nil) {
		if s == nil || s.Project == "" || s.Fingerprint == "" {
			return t, errors.New("必须先读取当前 Meshing 几何")
		}
		t.ExpectedProject = s.Project
		t.Fingerprint = s.Fingerprint
	}
	if phase == "apply" {
		if p == nil || p.ExpectedProject != t.ExpectedProject || p.Fingerprint != t.Fingerprint {
			return t, errors.New("网格建议与当前几何不匹配")
		}
		t.Plan = p
	}
	if phase == "export" || phase == "auto" || phase == "auto_onepass" || phase == "prepare" || phase == "publish_names" || phase == "publish_existing_sizing" {
		if phase == "auto" || phase == "auto_onepass" || phase == "prepare" || phase == "publish_names" || phase == "publish_existing_sizing" {
			// Strictly bind the automatic task to the geometry just inspected.
			// Worker rechecks before making any changes.
			t.BaseMM = 12.0
			if p != nil && p.GlobalMM >= 0.1 {
				t.BaseMM = p.GlobalMM
				t.ApplyGlobal = p.ApplyGlobal
			}
		}
		if phase == "export" || exportPath != "" {
			if !strings.EqualFold(filepath.Ext(exportPath), ".msh") {
				return t, errors.New("请选择 .msh 输出文件")
			}
			t.ExportPath = exportPath
			t.Overwrite = overwrite
		}
	}
	if (phase == "auto" || phase == "prepare" || phase == "publish_names" || phase == "publish_existing_sizing") && p != nil {
		t.Plan = p
		if phase == "prepare" || phase == "publish_existing_sizing" {
			t.AllowExistingMeshSizing = p.AllowExistingMeshSizing
		}
	}
	if (phase == "auto" || phase == "prepare" || phase == "publish_names" || phase == "publish_existing_sizing" || phase == "generate_reviewed") && (t.ExpectedProject == "" || t.Fingerprint == "") {
		return t, errors.New("一键自动网格必须先完成当前工程只读身份校验")
	}
	if err := w.InstallAssets(); err != nil {
		return t, err
	}
	if err := w.acquireTaskLease(t); err != nil {
		return t, err
	}
	if err := w.initializeTaskAttempt(t); err != nil {
		_ = w.removeTaskLeaseIfOwned(t.JobID)
		return t, err
	}
	return t, nil
}

var roles = []struct {
	Expr     *regexp.Regexp
	Category string
	Ratio    float64
}{
	{regexp.MustCompile(`(?i)inlet|入口|进气|(^|\W)in($|\W)`), "inlet", 0.33},
	{regexp.MustCompile(`(?i)outlet|出口|排气|(^|\W)out($|\W)`), "outlet", 0.33},
	{regexp.MustCompile(`(?i)wafer|晶圆|硅片|4inch|4.inch`), "wafer", 0.30},
	{regexp.MustCompile(`(?i)孔|hole|orifice`), "hole", 0.25},
	{regexp.MustCompile(`(?i)baffle|挡板|diffuser|匀气`), "baffle", 0.50},
}

func Propose(s Snapshot, mm float64, global bool) (Plan, error) {
	if mm < 0.05 || mm > 5000 {
		return Plan{}, errors.New("全局参考尺寸超出范围")
	}
	if s.Project == "" || s.Fingerprint == "" {
		return Plan{}, errors.New("当前几何信息不完整")
	}
	p := Plan{ExpectedProject: s.Project, Fingerprint: s.Fingerprint, GlobalMM: mm, MaxMM: mm * 2, ApplyGlobal: global, Controls: []Control{}, Review: []Control{}, Policy: "Only named face selections; no inference from unknown face types or historical IDs."}
	available := map[int]bool{}
	for _, f := range s.Faces {
		available[f.ID] = true
	}
	used := map[int]bool{}
	counts := map[string]int{}
	for _, n := range s.Named {
		var kind string
		var ratio float64
		for _, r := range roles {
			if r.Expr.MatchString(n.Name) {
				kind = r.Category
				ratio = r.Ratio
				break
			}
		}
		if kind == "" {
			continue
		}
		if len(n.IDs) == 0 {
			continue
		}
		uniq := map[int]bool{}
		valid := true
		for _, id := range n.IDs {
			if !available[id] || uniq[id] {
				valid = false
			}
			uniq[id] = true
		}
		if !valid {
			continue
		}
		c := Control{Name: n.Name, Category: kind, IDs: n.IDs, Count: len(n.IDs), MM: ratio * mm}
		for _, id := range n.IDs {
			if used[id] {
				c.SkipReason = "和已有区域面重叠，跳过"
				break
			}
		}
		c.MM, _ = strconv.ParseFloat(fmt.Sprintf("%.3f", c.MM), 64) // corrected below to use strconv 64 bits
		if c.MM < 0.15 {
			c.MM = 0.15
		}
		if c.SkipReason == "" {
			counts[kind]++
			c.ControlName = fmt.Sprintf("AutoMesh_%s_%d", kind, counts[kind])
			p.Controls = append(p.Controls, c)
			for _, id := range n.IDs {
				used[id] = true
			}
		}
		p.Review = append(p.Review, c)
	}
	return p, nil
}
func (w Workspace) LoadSnapshot() (Snapshot, error) {
	return readJSON[Snapshot](w.path("snapshot.json"))
}
func (w Workspace) LoadResult() (Result, error) { return readJSON[Result](w.path("last_result.json")) }
func (w Workspace) LoadTask() (Task, error)     { return readJSON[Task](w.path("job.json")) }
func (w Workspace) SavePlan(p Plan) error       { return writeJSON(w.path("plan.json"), p) }
func (w Workspace) WorkbenchProbe() (string, error) {
	p := w.path("Probe_Existing_Workbench.wbjn")
	txt := `# coding: utf-8
# Read-only diagnostic of the currently open Workbench GUI.
import json
items=[]
for s in GetAllSystems():
    try:
        name=str(s.Name)
    except Exception:
        continue
    try:
        label=str(s.DisplayText)
    except Exception:
        label=name
    mesh=False
    try:
        mesh=s.GetContainer(ComponentName='Mesh') is not None
    except Exception:
        pass
    items.append({'name':name,'label':label,'mesh':mesh})
with open(` + pyLiteral(w.path("wb_systems.json")) + `, 'w') as fp:
    fp.write(json.dumps(items,indent=2))
`
	return p, writeAtomic(p, []byte(txt))
}
