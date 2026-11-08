package main

import (
	"os"
	"strings"
	"testing"
)

func TestInstallPreflightCannotPartiallyReplaceWorker(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if e := os.WriteFile(w.path("workbench_bridge.py"), []byte("CUSTOM_KEEP_ME"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := w.InstallAssets(); e == nil {
		t.Fatal("custom bridge should prevent upgrade")
	}
	if _, e := os.Stat(w.path("meshing_worker.py")); !os.IsNotExist(e) {
		t.Fatal("worker was written before detecting custom bridge")
	}
	b, _ := os.ReadFile(w.path("workbench_bridge.py"))
	if string(b) != "CUSTOM_KEEP_ME" {
		t.Fatal("user bridge modified")
	}
}
func TestInstallRefusesUnknownLeaseBeforeWriting(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	id := strings.Repeat("c", 32)
	if e := writeJSON(w.taskLeasePath(), Task{JobID: id, Phase: "generate_reviewed"}); e != nil {
		t.Fatal(e)
	}
	if e := w.InstallAssets(); e == nil {
		t.Fatal("unknown lease not protected")
	}
	if _, e := os.Stat(w.path("meshing_worker.py")); !os.IsNotExist(e) {
		t.Fatal("worker written during unknown task")
	}
}
func TestInstallIdempotentAndOnlyClearsProvenTerminalLease(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if e := w.InstallAssets(); e != nil {
		t.Fatal(e)
	}
	if e := w.InstallAssets(); e != nil {
		t.Fatal(e)
	}
	// Same sources with an unknown job are not changed or cleared.
	id := strings.Repeat("e", 32)
	if e := writeJSON(w.taskLeasePath(), Task{JobID: id, Phase: "quality_check"}); e != nil {
		t.Fatal(e)
	}
	if e := w.InstallAssets(); e != nil {
		t.Fatal("idempotent install should not mutate anything:", e)
	}
	if _, e := os.Stat(w.taskLeasePath()); e != nil {
		t.Fatal("idempotent install removed unknown task")
	}
}
