package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestR72UpgradePreviousPortScripts(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	for _, name := range []string{"wb_cli.py", "wb_server_bootstrap.py"} {
		old, e := os.ReadFile(filepath.Join("testdata", "r71", name))
		if e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(w.path(name), old, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.InstallAssets(); e != nil {
		t.Fatal("R7.1 to R7.2 upgrade failed: ", e)
	}
	for _, name := range []string{"wb_cli.py", "wb_server_bootstrap.py"} {
		onDisk, e := os.ReadFile(w.path(name))
		if e != nil {
			t.Fatal(e)
		}
		wanted, e := os.ReadFile(filepath.Join("assets", name))
		if e != nil {
			t.Fatal(e)
		}
		if string(onDisk) != string(wanted) {
			t.Fatalf("asset %s not updated", name)
		}
	}
	if e := w.InstallAssets(); e != nil {
		t.Fatal("repeat upgrade not idempotent: ", e)
	}
}
func TestR72NeverOverwritesCustomServerFinder(t *testing.T) {
	if e := checkGeneratedAssetReplacement("wb_server_bootstrap.py", []byte("custom code"), []byte("new code")); e == nil {
		t.Fatal("modified script overwritten")
	}
}
