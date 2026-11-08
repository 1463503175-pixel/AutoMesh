package main

import (
	"os"
	"strings"
	"testing"
)

func TestAutoPortRecoveryDoesNotAssumeFixedRangeOrInsecure(t *testing.T) {
	b, e := os.ReadFile("main_windows.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	for _, expected := range []string{"a.discoverExistingServerPorts(preferred)", `a.security = "auto"`, "a.bootstrapExistingWorkbench()"} {
		if !strings.Contains(s, expected) {
			t.Fatalf("missing port recovery: %s", expected)
		}
	}
	if strings.Contains(s, `a.security = "insecure" // Explicitly local only`) {
		t.Fatal("old fixed insecure setting restored")
	}
}
