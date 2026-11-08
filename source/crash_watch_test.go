package main

import (
	"strings"
	"testing"
)

// The process could exit before ACT could write its PID. This must never be
// guessed as a crash or used to delete a lease.
func TestCrashWatchUnknownProcessDoesNotPretendCrash(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	id := strings.Repeat("a", 32)
	if reason, crashed := w.meshingProcessCrash(id); crashed || reason != "" {
		t.Fatalf("missing receipt falsely treated as exit: %q", reason)
	}
	if err := writeJSON(w.path("job_progress_"+id+".json"), ActProgress{
		JobID: strings.Repeat("b", 32), ProcessPID: 1234, ProcessStartedFiletime: "100",
	}); err != nil {
		t.Fatal(err)
	}
	if _, crashed := w.meshingProcessCrash(id); crashed {
		t.Fatal("wrong job PID observed")
	}
}

func TestCrashWatchOldLeasePreserved(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	id := strings.Repeat("a", 32)
	if err := writeJSON(w.taskLeasePath(), Task{JobID: id, Phase: "publish_existing_sizing"}); err != nil {
		t.Fatal(err)
	}
	if _, crashed := w.meshingProcessCrash(id); crashed {
		t.Fatal("false positive")
	}
	lease, err := readJSON[Task](w.taskLeasePath())
	if err != nil || lease.JobID != id {
		t.Fatal("crash observation must not clear lease")
	}
}
