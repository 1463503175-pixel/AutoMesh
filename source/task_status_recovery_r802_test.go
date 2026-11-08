package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestR802LivePreviewRecoveryUsesExactJobBoundPlan(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	jobID := strings.Repeat("c", 32)
	lease := Task{JobID: jobID, Phase: "warning_refine_preview", ExpectedProject: "project-A", Fingerprint: "fingerprint-A"}
	if err := writeJSON(w.taskLeasePath(), lease); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.path("snapshot.json"), Snapshot{Project: lease.ExpectedProject, Fingerprint: lease.Fingerprint}); err != nil {
		t.Fatal(err)
	}
	plan := WarningPlan{JobID: jobID, SchemaVersion: 1, Status: "no_safe_candidates", PlanID: "plan-1234567890", Project: lease.ExpectedProject, Fingerprint: lease.Fingerprint, WarningCount: 4, CoveredCount: 0, UncoveredCount: 4, MeshElements: 3803875}
	if err := writeJSON(w.path("warning_refinement_plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	progress := ActProgress{JobID: jobID, Stage: "终态结果全部写入失败", Detail: "simulated ACT result receipt failure", Timestamp: float64(time.Now().Add(-15*time.Second).UnixNano()) / 1e9}
	if err := writeJSON(w.path("job_progress_"+jobID+".json"), progress); err != nil {
		t.Fatal(err)
	}

	status, terminal := remoteJobObservation(w, jobID)
	if status != "act_terminal" || terminal == nil {
		t.Fatalf("expected live terminal recovery, got status=%q terminal=%v", status, terminal)
	}
	if terminal.Status != "ok" || terminal.Phase != "warning_refine_preview" {
		t.Fatalf("expected recovered preview success, got %+v", terminal)
	}
	if _, err := os.Stat(w.warningPreviewReceiptPath()); err != nil {
		t.Fatalf("preview receipt not created: %v", err)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatalf("live observation must leave lease release to taskRun: %v", err)
	}
	if err := w.releaseTaskLease(jobID); err != nil {
		t.Fatalf("taskRun could not release recovered terminal lease: %v", err)
	}
}

func TestR802PreviewRecoveryRejectsPlanFromAnotherJob(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	jobID := strings.Repeat("d", 32)
	lease := Task{JobID: jobID, Phase: "warning_refine_preview", ExpectedProject: "project-A", Fingerprint: "fingerprint-A"}
	if err := writeJSON(w.taskLeasePath(), lease); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.path("snapshot.json"), Snapshot{Project: lease.ExpectedProject, Fingerprint: lease.Fingerprint}); err != nil {
		t.Fatal(err)
	}
	plan := WarningPlan{JobID: strings.Repeat("e", 32), SchemaVersion: 1, Status: "no_safe_candidates", PlanID: "plan-1234567890", Project: lease.ExpectedProject, Fingerprint: lease.Fingerprint, WarningCount: 4, CoveredCount: 0, UncoveredCount: 4, MeshElements: 3803875}
	if err := writeJSON(w.path("warning_refinement_plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	progress := ActProgress{JobID: jobID, Stage: "终态结果全部写入失败", Detail: "simulated ACT result receipt failure", Timestamp: float64(time.Now().Add(-15*time.Second).UnixNano()) / 1e9}
	if err := writeJSON(w.path("job_progress_"+jobID+".json"), progress); err != nil {
		t.Fatal(err)
	}

	status, terminal := remoteJobObservation(w, jobID)
	if status != "act_terminal" || terminal == nil {
		t.Fatalf("expected recovered terminal error, got status=%q terminal=%v", status, terminal)
	}
	if terminal.Status != "error" {
		t.Fatalf("must not invent success for another job's plan: %+v", terminal)
	}
	if _, err := os.Stat(w.warningPreviewReceiptPath()); !os.IsNotExist(err) {
		t.Fatalf("mismatched plan must not create preview receipt: %v", err)
	}
}
