package main

import (
	"os"
	"strings"
	"testing"
)

func TestStartupWritesReadyDiagnosticWithoutTouchingTasks(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if err := w.InitializeScriptsAndRecord(); err != nil {
		t.Fatal(err)
	}
	d, err := readJSON[startupDiagnostic](w.path("startup_diagnostic.json"))
	if err != nil || d.Status != "ready" || d.Workspace != w.Dir {
		t.Fatalf("status %#v, %v", d, err)
	}
	if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
		t.Fatal("must not invent or clear task lease")
	}
	if err := w.InitializeScriptsAndRecord(); err != nil {
		t.Fatal("idempotent:", err)
	}
}

func TestStartupPreservesUnconfirmedTaskAndShowsActualError(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if err := w.InstallAssets(); err != nil {
		t.Fatal(err)
	}
	// Known R6.2 worker is allowed for upgrade, but an unconfirmed task blocks it.
	old, err := os.ReadFile("assets/meshing_worker.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(old) < 1 {
		t.Fatal("empty worker")
	}
	// Unknown customization forces an update which must not bypass task lock.
	if err := os.WriteFile(w.path("meshing_worker.py"), []byte("modified user script"), 0600); err != nil {
		t.Fatal(err)
	}
	lease := Task{JobID: strings.Repeat("a", 32), Phase: "publish_existing_sizing"}
	if err := writeJSON(w.taskLeasePath(), lease); err != nil {
		t.Fatal(err)
	}
	got := w.InitializeScriptsAndRecord()
	if got == nil {
		t.Fatal("expected explicit init failure")
	}
	d, err := readJSON[startupDiagnostic](w.path("startup_diagnostic.json"))
	if err != nil || d.Status != "failed" || d.Error == "" {
		t.Fatalf("missing diagnostic %#v %v", d, err)
	}
	b, err := os.ReadFile(w.path("meshing_worker.py"))
	if err != nil || string(b) != "modified user script" {
		t.Fatal("unsafe overwrite")
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("task lease was removed")
	}
}

func TestStartupWithOldLeaseBlocksUpdateAndDoesNotDeleteLease(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	id := strings.Repeat("b", 32)
	if err := writeJSON(w.taskLeasePath(), Task{JobID: id, Phase: "publish_existing_sizing"}); err != nil {
		t.Fatal(err)
	}
	err := w.InitializeScriptsAndRecord()
	if err == nil || !strings.Contains(err.Error(), "未确认结束") {
		t.Fatal("incorrect error:", err)
	}
	if _, err = os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("lease lost")
	}
	d, _ := readJSON[startupDiagnostic](w.path("startup_diagnostic.json"))
	if d.Status != "failed" {
		t.Fatal(d)
	}
}
