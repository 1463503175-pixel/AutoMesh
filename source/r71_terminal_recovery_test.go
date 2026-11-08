package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeTestLease(t *testing.T, w Workspace, letter string) Task {
	t.Helper()
	lease := Task{JobID: strings.Repeat(letter, 32), Phase: "warning_refine_preview"}
	if err := w.acquireTaskLease(lease); err != nil {
		t.Fatal(err)
	}
	return lease
}

func TestR71TerminalProgressErrorReleasesLeaseAndPreservesPriorResult(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	prior := Result{JobID: strings.Repeat("a", 32), Phase: "quality_check", Status: "ok"}
	if err := writeJSON(w.path("last_result.json"), prior); err != nil {
		t.Fatal(err)
	}
	lease := makeTestLease(t, w, "b")
	progress := ActProgress{JobID: lease.JobID, Stage: "任务失败", Detail: "全部警告点均不适合安全自动细化", Timestamp: float64(time.Now().Add(-20 * time.Second).Unix())}
	if err := writeJSON(w.path("job_progress_"+lease.JobID+".json"), progress); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
		t.Fatalf("terminal lease remains: %v", err)
	}
	out, err := w.LoadResult()
	if err != nil {
		t.Fatal(err)
	}
	if out.JobID != lease.JobID || out.Status != "error" || !strings.Contains(out.Error, "全部警告点") {
		t.Fatalf("bad reconstructed result: %+v", out)
	}
	old, err := readJSON[Result](w.path("job_" + prior.JobID + ".json"))
	if err != nil || old.JobID != prior.JobID {
		t.Fatalf("prior result not preserved: %+v %v", old, err)
	}
}

func TestR71UnknownAndUnfinishedProgressNeverUnlock(t *testing.T) {
	for _, stage := range []string{"定位警告单元", "任务完成", "等待任务状态", "任务失败"} {
		t.Run(stage, func(t *testing.T) {
			w := Workspace{Dir: t.TempDir()}
			lease := makeTestLease(t, w, "c")
			at := time.Now().Add(-time.Minute)
			if stage == "任务失败" {
				at = time.Now() /* too recent to recover */
			}
			if err := writeJSON(w.path("job_progress_"+lease.JobID+".json"), ActProgress{JobID: lease.JobID, Stage: stage, Timestamp: float64(at.Unix())}); err != nil {
				t.Fatal(err)
			}
			if err := w.ReconcileFinishedLease(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(w.taskLeasePath()); err != nil {
				t.Fatalf("unconfirmed lease was unlocked: %v", err)
			}
		})
	}
}

func TestR71PerJobReceiptCanRepairLastResultAndRelease(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	lease := makeTestLease(t, w, "d")
	r := Result{JobID: lease.JobID, Phase: lease.Phase, Status: "error", Error: "ACT confirmed failure"}
	if err := writeJSON(w.path("job_"+lease.JobID+".json"), r); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
		t.Fatalf("job-specific receipt did not release lease: %v", err)
	}
	restored, err := w.LoadResult()
	if err != nil || restored.Error != r.Error {
		t.Fatalf("generic result not restored: %+v %v", restored, err)
	}
}

func TestR71MismatchedTerminalMarkerDoesNotRelease(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	lease := makeTestLease(t, w, "e")
	p := ActProgress{JobID: strings.Repeat("f", 32), Stage: "任务失败", Timestamp: float64(time.Now().Add(-20 * time.Second).Unix())}
	if err := writeJSON(filepath.Join(w.Dir, "job_progress_"+lease.JobID+".json"), p); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatalf("wrong job marker released the lease: %v", err)
	}
}

func TestR71R7WorkerUpgradeHashRegistered(t *testing.T) {
	oldHash := "a852ad605154c97a24d5f28a785dd4a5acd0646c55a5cb17eb211a88dae126f3"
	if !knownLegacyWorkerHashes[oldHash] {
		t.Fatal("R7 installed worker would block upgrade")
	}
}

func TestR71StatusReportsRecoveredTerminalInsteadOfUnknown(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	lease := makeTestLease(t, w, "f")
	if err := writeJSON(w.path("job_progress_"+lease.JobID+".json"), ActProgress{JobID: lease.JobID, Stage: "任务失败", Detail: "unsafe warning locations", Timestamp: float64(time.Now().Add(-20 * time.Second).Unix())}); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	status := w.taskStatus()
	if !strings.Contains(status, "恢复终态") || !strings.Contains(status, "unsafe warning locations") {
		t.Fatalf("lost recovered failure detail: %s", status)
	}
}
