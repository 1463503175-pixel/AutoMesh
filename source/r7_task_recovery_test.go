package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR7CancellationIsDurableAndScoped(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	job := Task{JobID: strings.Repeat("a", 32), Phase: "warning_refine_preview"}
	if err := w.acquireTaskLease(job); err != nil {
		t.Fatal(err)
	}
	if err := w.RequestTaskCancel(strings.Repeat("b", 32), "operator"); err == nil {
		t.Fatal("accepted unrelated cancellation")
	}
	if err := w.RequestTaskCancel(job.JobID, "operator"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(w.cancelPath(job.JobID))
	if err != nil {
		t.Fatal(err)
	}
	var req TaskCancelRequest
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatal(err)
	}
	if req.JobID != job.JobID || req.RequestedAt == "" {
		t.Fatalf("bad durable cancellation: %+v", req)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("cancel wrongly removed task lock")
	}
}
func TestR7ReconcileOnlyVerifiedTerminalLease(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	job := Task{JobID: strings.Repeat("b", 32), Phase: "quality_check"}
	if err := w.acquireTaskLease(job); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("unknown task was cleared")
	}
	if err := writeJSON(filepath.Join(w.Dir, "last_result.json"), Result{JobID: job.JobID, Phase: job.Phase, Status: "cancelled"}); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
		t.Fatal("terminal task was not unlocked", err)
	}
}
func TestR7EmptyCandidateDiagnosticNotPublishable(t *testing.T) {
	s := &Snapshot{Project: "E:/ALD", Fingerprint: "xyz"}
	p := WarningPlan{SchemaVersion: 1, Status: "no_safe_candidates", PlanID: "safe", Project: s.Project, Fingerprint: s.Fingerprint, WarningCount: 13, UncoveredCount: 13, RejectedGroups: []WarningRejection{{ElementIDs: []int{101}, Reason: "influence_radius_exceeds_limit"}}}
	if err := validateWarningDiagnostic(p, s, "safe"); err != nil {
		t.Fatal("diagnostic rejected", err)
	}
	if err := validateWarningPlan(p, s, "safe"); err == nil {
		t.Fatal("no-candidate diagnostic accidentally allowed Sizing")
	}
}
