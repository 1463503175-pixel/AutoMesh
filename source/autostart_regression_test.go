package main

import (
	"os"
	"strings"
	"testing"
)

func TestStartupAutomaticallyConnects(t *testing.T) {
	b, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, x := range []string{"case WM_APPINIT:", "ui.connect(true)", "a.connect(false)", "if !wasAutomatic {", "a.bootstrapExistingWorkbench()"} {
		if !strings.Contains(s, x) {
			t.Errorf("missing automatic start behavior: %s", x)
		}
	}
	if strings.Contains(s, "launch_workbench()") {
		t.Error("must not launch new Workbench")
	}
}
