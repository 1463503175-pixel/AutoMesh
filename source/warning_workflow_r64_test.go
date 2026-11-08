package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestR64WarningPreviewDoesNotDependOnLastQualityCheck(t *testing.T) {
	dir := t.TempDir()
	w := Workspace{Dir: dir}
	snap := &Snapshot{Project: "E:/ALD/one", Fingerprint: "geo"}
	plan := WarningPlan{SchemaVersion: 1, Status: "ready", PlanID: "p123", Project: snap.Project, Fingerprint: snap.Fingerprint,
		WarningCount: 3, CoveredCount: 2, UncoveredCount: 1, MeshElements: 4575877,
		Clusters: []WarningHotspot{{BodyID: 1, CentreMM: []float64{1, 2, 3}, RadiusMM: 2, SizeMM: 1, ElementIDs: []int{101, 102}}}}
	if err := writeJSON(w.path("warning_refinement_plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"plan_id": "p123", "warning_count": 3})
	if err := w.RecordWarningPreview(Result{JobID: strings.Repeat("a", 32), Phase: "warning_refine_preview", Status: "ok", Summary: b}, snap); err != nil {
		t.Fatal(err)
	}
	// A later unrelated quality result does not invalidate the reviewed plan.
	if err := writeJSON(w.path("last_result.json"), Result{JobID: strings.Repeat("b", 32), Phase: "quality_check", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	actual, err := w.ReadReviewedWarningPlan(snap)
	if err != nil || actual.PlanID != "p123" {
		t.Fatalf("lost preview after quality check: %+v %v", actual, err)
	}
	if _, err = w.ReadReviewedWarningPlan(&Snapshot{Project: snap.Project, Fingerprint: "changed"}); err == nil {
		t.Fatal("accepted stale geometry")
	}
	if err = w.InvalidateWarningPreview(); err != nil {
		t.Fatal(err)
	}
	if _, err = w.ReadReviewedWarningPlan(snap); err == nil {
		t.Fatal("accepted invalidated review")
	}
}

func TestR64PreviewReceiptRequiresWorkerSuccessAndMatchingPlan(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	snap := &Snapshot{Project: "E:/ALD/one", Fingerprint: "geo"}
	b, _ := json.Marshal(map[string]any{"plan_id": "p123"})
	for _, v := range []Result{{JobID: "a", Phase: "quality_check", Status: "ok", Summary: b}, {JobID: "a", Phase: "warning_refine_preview", Status: "error", Summary: b}} {
		if err := w.RecordWarningPreview(v, snap); err == nil {
			t.Fatal("accepted failed/non-preview job")
		}
	}
	if err := w.RecordWarningPreview(Result{JobID: "a", Phase: "warning_refine_preview", Status: "ok", Summary: b}, snap); err == nil {
		t.Fatal("accepted preview without plan")
	}
	if _, err := os.Stat(w.warningPreviewReceiptPath()); !os.IsNotExist(err) {
		t.Fatal("unexpected review receipt")
	}
}

func TestR64WarningACTStalledRemainsUnconfirmed(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	id := strings.Repeat("c", 32)
	if err := writeJSON(w.path("job_progress_"+id+".json"), ActProgress{JobID: id, Stage: "locating warning", Timestamp: float64(time.Now().Add(-180 * time.Second).Unix())}); err != nil {
		t.Fatal(err)
	}
	state, result := remoteJobObservation(w, id)
	if state != "act_stalled" || result != nil {
		t.Fatalf("%s %+v", state, result)
	}
	if err := writeJSON(w.taskLeasePath(), Task{JobID: id, Phase: "warning_refine_preview"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("must not auto-clear lease")
	}
	if err := writeJSON(w.path("last_result.json"), Result{JobID: id, Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	state, result = remoteJobObservation(w, id)
	if state != "act_terminal" || result == nil {
		t.Fatal("terminal result must win over stale progress")
	}
}

func TestR64R632WorkerUpgradeRecognized(t *testing.T) {
	if !knownLegacyWorkerHashes["0be57e7b331730cd06f41de933486b34f89888f10689f537df1c337fd41e7ad1"] {
		t.Fatal("R6.3.2 upgrade missing")
	}
}
