package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNoGeometryGuessing(t *testing.T) {
	s := Snapshot{Project: "abc", Fingerprint: "hash", Faces: []Face{{ID: 1}, {ID: 2}, {ID: 3}}, Named: []Named{{Name: "入口_A", IDs: []int{1}}, {Name: "晶圆", IDs: []int{2}}, {Name: "unknown", IDs: []int{3}}}}
	p, e := Propose(s, 12, false)
	if e != nil || len(p.Controls) != 2 {
		t.Fatalf("plan %v %v", p, e)
	}
	if p.Controls[0].MM != 3.96 {
		t.Fatalf("unexpected mm %v", p.Controls[0].MM)
	}
}
func TestOverwriteAndTaskGuard(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	_, err := w.Submit("generate", nil, nil, "", false)
	if err == nil {
		t.Fatal("should require snapshot")
	}
	if _, err = w.Submit("inspect", nil, nil, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(w.path("Run_In_Existing_Meshing.py")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(w.path("job.json"))
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m["phase"] != "inspect" {
		t.Fatal("bad task")
	}
}
func TestEmbeddedRunner(t *testing.T) {
	w := Workspace{Dir: filepath.Join(t.TempDir(), "run")}
	if e := w.InstallAssets(); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(w.path("Run_In_Existing_Meshing.py"))
	if e != nil || !strings.Contains(string(b), "execfile(") {
		t.Fatal(e)
	}
}
func TestDuplicatedFace(t *testing.T) {
	s := Snapshot{Project: "p", Fingerprint: "f", Faces: []Face{{ID: 1}}, Named: []Named{{Name: "inlet", IDs: []int{1}}, {Name: "hole", IDs: []int{1}}}}
	p, e := Propose(s, 12, false)
	if e != nil || len(p.Controls) != 1 || len(p.Review) != 2 || p.Review[1].SkipReason == "" {
		t.Fatalf("overlap not rejected %v", p)
	}
}

func TestAutoRequiresPinnedGeometry(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	p := &Plan{GlobalMM: 12}
	if _, err := w.Submit("auto", nil, p, "", false); err == nil {
		t.Fatal("auto must never execute on unverified current geometry")
	}
	snapshot := &Snapshot{Project: "E:/sim/Model_files/", Fingerprint: "abc123"}
	job, err := w.Submit("auto", snapshot, p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if job.ExpectedProject != snapshot.Project || job.Fingerprint != snapshot.Fingerprint {
		t.Fatalf("project binding lost: %+v", job)
	}
}
func TestAutoRefusesOverwrite(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	f := w.path("existing.msh")
	if e := os.WriteFile(f, []byte("mesh"), 0644); e != nil {
		t.Fatal(e)
	}
	// The GUI preflights file existence; the worker independently rejects
	// anything that exists regardless of the overwrite flag.
	job, err := w.Submit("auto", &Snapshot{Project: "p", Fingerprint: "h"}, &Plan{GlobalMM: 12}, f, false)
	if err != nil {
		t.Fatal(err)
	}
	if job.Overwrite {
		t.Fatal("automatic mode must not overwrite")
	}
}

func TestOnePassSubmission(t *testing.T) {
	dir := t.TempDir()
	ws := Workspace{Dir: dir}
	_, err := ws.Submit("auto_onepass", nil, &Plan{GlobalMM: 12, ApplyGlobal: false}, "", false)
	if err != nil {
		t.Fatalf("one-pass must not require stale prior snapshot: %v", err)
	}
	task, err := ws.LoadTask()
	if err != nil {
		t.Fatal(err)
	}
	if task.ExpectedProject != "" || task.Fingerprint != "" {
		t.Fatal("onepass must inspect inside same ACT call")
	}
	if task.BaseMM != 12 {
		t.Fatal("onepass mm lost")
	}
}

func TestWaitResultDoesNotAcceptStaleOrIncomplete(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	job := strings.Repeat("a", 32)
	stale := strings.Repeat("b", 32)
	_ = writeJSON(w.path("last_result.json"), Result{JobID: stale, Phase: "auto_onepass", Status: "ok"})
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(25 * time.Millisecond)
		_ = writeJSON(w.path("last_result.json"), Result{JobID: job, Phase: "auto_onepass", Status: "in_progress"})
		time.Sleep(25 * time.Millisecond)
		_ = writeJSON(w.path("last_result.json"), Result{JobID: job, Phase: "auto_onepass", Status: "ok"})
	}()
	result, err := w.WaitResult(job, time.Second, 10*time.Millisecond, nil)
	<-done
	if err != nil || result.Status != "ok" || result.JobID != job {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestWaitResultNeverAcceptsOldResult(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	_ = writeJSON(w.path("last_result.json"), Result{JobID: strings.Repeat("b", 32), Status: "ok"})
	_, err := w.WaitResult(strings.Repeat("c", 32), 30*time.Millisecond, 10*time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "禁止重复提交") {
		t.Fatalf("missing strong timeout warning: %v", err)
	}
}

func TestPreferAsciiSharedWorkspace(t *testing.T) {
	ascii := filepath.Join(t.TempDir(), "ProgramData")
	t.Setenv("ProgramData", ascii)
	t.Setenv("PUBLIC", filepath.Join(t.TempDir(), "公共"))
	w, err := DefaultWorkspace()
	if err != nil || !asciiPath(w.Dir) {
		t.Fatalf("workspace = %q, %v", w.Dir, err)
	}
	if _, err = os.Stat(w.Dir); err != nil {
		t.Fatal(err)
	}
}

func TestLeasePreventsOverwritingLiveTask(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Submit("inspect", nil, nil, "", false); err == nil || !strings.Contains(err.Error(), "未确认结束") {
		t.Fatalf("should block concurrent task: %v", err)
	}
	current, err := w.LoadTask()
	if err != nil || current.JobID != task.JobID {
		t.Fatalf("job.json overwritten: %+v %v", current, err)
	}
	if err := w.releaseTaskLease(task.JobID); err == nil {
		t.Fatal("released active task without terminal result")
	}
	if err := writeJSON(w.path("last_result.json"), Result{JobID: task.JobID, Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	second, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil || second.JobID == task.JobID {
		t.Fatalf("finished job should be released only on confirmed terminal status: %v", err)
	}
}

func TestLeaseNeverFreesOnUnrelatedOrInProgressResult(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []Result{{JobID: task.JobID, Status: "in_progress"}, {JobID: strings.Repeat("b", 32), Status: "ok"}} {
		if e := writeJSON(w.path("last_result.json"), r); e != nil {
			t.Fatal(e)
		}
		if _, e := w.Submit("inspect", nil, nil, "", false); e == nil {
			t.Fatalf("released task for %+v", r)
		}
	}
}

func TestAbandonedTaskRequiresExplicitApprovalAndArchives(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.Submit("inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := w.AbandonTaskLease(""); e == nil {
		t.Fatal("missing explicit approval allowed")
	}
	if _, e := os.Stat(w.taskLeasePath()); e != nil {
		t.Fatal("task lock vanished without consent", e)
	}
	audit, e := w.AbandonTaskLease("I_CONFIRMED_MESHING_STOPPED")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(audit); e != nil {
		t.Fatal("old job not archived", e)
	}
	if _, e := os.Stat(w.taskLeasePath()); !os.IsNotExist(e) {
		t.Fatal("old lock not released")
	}
	old, e := readJSON[map[string]any](audit)
	if e != nil || old["reason"] == nil || task.JobID == "" {
		t.Fatalf("audit incomplete: %v %v", e, old)
	}
}

func TestExportRequiresExplicitQualityAcknowledgement(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	s := &Snapshot{Project: "E:/model/FFF", Fingerprint: "digest"}
	job, err := w.Submit("export", s, nil, "E:/results/test.msh", false)
	if err != nil {
		t.Fatal(err)
	}
	if job.QualityReviewAcknowledged {
		t.Fatal("acknowledgement must not be default")
	}
	if err := writeJSON(w.path("last_result.json"), Result{JobID: job.JobID, Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	next, err := w.Submit("export", s, nil, "E:/results/test2.msh", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !next.QualityReviewAcknowledged {
		t.Fatal("explicit acknowledgement not recorded")
	}
}

func TestPublishNamesPhaseAndExplicitMeshedSizingConsent(t *testing.T) {
	s := &Snapshot{Project: "E:/model/FFF", Fingerprint: "hash"}
	w := Workspace{Dir: t.TempDir()}
	names, err := w.Submit("publish_names", s, &Plan{GlobalMM: 12}, "", false)
	if err != nil || names.AllowExistingMeshSizing {
		t.Fatalf("names submit %+v err %v", names, err)
	}
	_ = os.Remove(w.taskLeasePath()) // simulate completed task before next submit
	sizing, err := w.Submit("prepare", s, &Plan{GlobalMM: 12, AllowExistingMeshSizing: true}, "", false)
	if err != nil || !sizing.AllowExistingMeshSizing {
		t.Fatalf("sizing consent not transmitted %+v err %v", sizing, err)
	}
}

func TestACTErrorFriendlyForExistingMesh(t *testing.T) {
	text := "Traceback (most recent call last):\nRuntimeError: Existing mesh: explicit Sizing publication consent required"
	message := friendlyACTError(text)
	if strings.Contains(message, "Traceback") || !strings.Contains(message, "仅发布命名") {
		t.Fatalf("unfriendly message: %s", message)
	}
}

func TestProgressEventNeverInventsGenerateMeshPercent(t *testing.T) {
	m := ActProgress{JobID: strings.Repeat("a", 32), Stage: "ANSYS 正在生成网格", Detail: "无法获取准确百分比"}
	if got := displayActProgress(m); !strings.HasPrefix(got, "I|") {
		t.Fatal(got)
	}
	percent := 75
	m.Percent = &percent
	if got := displayActProgress(m); !strings.HasPrefix(got, "P75|") {
		t.Fatal(got)
	}
	bad := 110
	m.Percent = &bad
	if got := displayActProgress(m); !strings.HasPrefix(got, "I|") {
		t.Fatal("Invalid percent accepted", got)
	}
}

func TestSubmitExistingNamedSizingUsesExactExplicitNameSizeMap(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	snap := Snapshot{Project: "E:/test/FFF", Fingerprint: "valid-fingerprint"}
	p := Plan{GlobalMM: 8, ExistingNamedSizes: map[string]float64{"AM_Area_Cylinder_01": 1.5, "4inch": 3.5}, AllowExistingMeshSizing: true}
	task, err := w.Submit("publish_existing_sizing", &snap, &p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if task.Plan == nil || len(task.Plan.ExistingNamedSizes) != 2 {
		t.Fatalf("lost chosen names: %+v", task.Plan)
	}
	if !task.AllowExistingMeshSizing {
		t.Fatal("lost existing mesh risk approval")
	}
	stored, err := readJSON[Task](w.path("job.json"))
	if err != nil {
		t.Fatal(err)
	}
	if stored.Plan.ExistingNamedSizes["AM_Area_Cylinder_01"] != 1.5 {
		t.Fatalf("serialized map lost: %+v", stored.Plan)
	}
	if stored.ApplyGlobal {
		t.Fatal("publishing named Sizing must not change global size")
	}
}

func TestNewReadOnlyQualityAndChineseOperationsCanBeSubmitted(t *testing.T) {
	for _, phase := range []string{"quality_check", "rename_auxiliary"} {
		w := Workspace{Dir: t.TempDir()}
		snap := &Snapshot{Project: "C:/mesh/project", Fingerprint: "identity", Mesh: map[string]any{"Elements": 4000}}
		task, err := w.Submit(phase, snap, nil, "", false)
		if err != nil {
			t.Fatalf("%s must be in task whitelist: %v", phase, err)
		}
		if task.Phase != phase {
			t.Fatalf("wrong phase: %+v", task)
		}
	}
}

func TestSubmitForSystemPersistsMeshIdentity(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	task, err := w.SubmitForSystem("FFF", "inspect", nil, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if task.MeshSystem != "FFF" {
		t.Fatalf("returned task lost Mesh system: %+v", task)
	}
	job, err := w.LoadTask()
	if err != nil || job.MeshSystem != "FFF" {
		t.Fatalf("job.json Mesh system = %q, err=%v", job.MeshSystem, err)
	}
	lease, err := readJSON[Task](w.taskLeasePath())
	if err != nil || lease.MeshSystem != "FFF" {
		t.Fatalf("active task Mesh system = %q, err=%v", lease.MeshSystem, err)
	}
}
