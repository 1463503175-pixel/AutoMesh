package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestR7UpgradeFromR632Files(t *testing.T) {
	source := "../AutoMesh_R6_3_2_AutoStart_Source/assets"
	if _, e := os.Stat(source); os.IsNotExist(e) {
		t.Skip("old release fixture not present in distribution")
	}
	w := Workspace{Dir: t.TempDir()}
	for _, name := range []string{"meshing_worker.py", "warning_refinement.py", "workbench_bridge.py", "wb_cli.py", "geometry_rules.py", "wb_server_bootstrap.py", "start_wb_server_uia.ps1"} {
		b, e := os.ReadFile(filepath.Join(source, name))
		if e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(w.Dir, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.InitializeScriptsAndRecord(); e != nil {
		t.Fatal("failed to upgrade original R6.3.2: ", e)
	}
	if e := w.InstallAssets(); e != nil {
		t.Fatal("second install must be idempotent: ", e)
	}
}
