package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLegacyWarningManifest(t *testing.T, w Workspace, project, fingerprint string) {
	t.Helper()
	manifest := map[string]any{
		"schema_version": 2,
		"project":        project,
		"fingerprint":    fingerprint,
		"controls": []any{map[string]any{
			"candidate_id":      "aaaaaaaaaaaaaaaaaaaa",
			"body_id":           1,
			"centre_mm":         []float64{1, 2, 3},
			"radius_mm":         2.0,
			"size_mm":           1.0,
			"cell_budget_proxy": 34,
		}},
	}
	if err := writeJSON(w.path("warning_refinement_managed.json"), manifest); err != nil {
		t.Fatal(err)
	}
}

func TestR803ArchivedWarningManifestIgnoredOnlyWhenNoLiveQRControls(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	writeLegacyWarningManifest(t, w, "old-project", "old-geometry")

	// Existing ordinary/legacy named Sizing does not establish ownership of
	// the reserved quality-hotspot QR namespace.
	s := &Snapshot{Project: "current-project", Fingerprint: "current-geometry", Controls: []map[string]any{
		{"name": "AutoMesh_NS_AM_Inlet", "category": "Sizing"},
		{"name": "ManualSizing_01", "category": "Sizing"},
	}}
	owned, budget, err := w.warningOwnedCandidates(s)
	if err != nil || len(owned) != 0 || budget != 0 {
		t.Fatalf("archived manifest should not block current model with no live QR controls: owned=%v budget=%d err=%v", owned, budget, err)
	}
	if err := w.reconcileReviewLegacyPublished(s, WarningPlan{}, owned); err != nil {
		t.Fatalf("legacy reconciliation should ignore verified archived manifest: %v", err)
	}
}

func TestR803StaleWarningManifestBlocksPublishWhenQRControlIsLive(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	writeLegacyWarningManifest(t, w, "old-project", "old-geometry")
	s := &Snapshot{Project: "current-project", Fingerprint: "current-geometry", Controls: []map[string]any{
		{"name": "AutoMesh_QR_0123456789ab", "category": "Sizing"},
	}}
	if _, _, err := w.warningOwnedCandidates(s); err == nil {
		t.Fatal("must not silently ignore a stale manifest while a live QR control exists")
	}
	if err := staleWarningManifestDisposition(&Snapshot{Project: "p", Fingerprint: "f"}); err == nil {
		t.Fatal("nil controls list must be treated as unknown, not proof of absence")
	}
}

func detailedWarningSnapshot(project, fingerprint string) *Snapshot {
	return &Snapshot{
		Project: project, Fingerprint: fingerprint,
		Bodies: []Body{{ID: 1}}, ControlsComplete: true, WarningCoordinateSystemsComplete: true,
		Controls: []map[string]any{{
			"name": "AutoMesh_QR_0123456789ab", "category": "Sizing", "object_id": 101,
			"is_sizing": true, "type": "SphereOfInfluence", "suppressed": false,
			"body_ids": []any{float64(1)}, "size_mm": float64(1), "radius_mm": float64(2),
			"centre_mm":              []any{float64(1), float64(2), float64(3)},
			"coordinate_system_name": "AutoMesh_QR_CS_0123456789ab", "coordinate_system_object_id": float64(201),
		}},
		WarningCoordinateSystems: []map[string]any{{"name": "AutoMesh_QR_CS_0123456789ab", "object_id": float64(201)}},
	}
}

func TestR803CurrentWarningManifestStillCountsPublishedControls(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	manifest := map[string]any{"schema_version": 2, "project": "P", "fingerprint": "F", "controls": []any{map[string]any{
		"candidate_id": "aaaaaaaaaaaaaaaaaaaa", "name": "AutoMesh_QR_0123456789ab", "object_id": 101,
		"coordinate_system": "AutoMesh_QR_CS_0123456789ab", "cs_object_id": 201,
		"body_id": 1, "centre_mm": []float64{1, 2, 3}, "radius_mm": 2.0, "size_mm": 1.0, "cell_budget_proxy": 34,
	}}}
	if err := writeJSON(w.path("warning_refinement_managed.json"), manifest); err != nil {
		t.Fatal(err)
	}
	s := detailedWarningSnapshot("P", "F")
	owned, budget, err := w.warningOwnedCandidates(s)
	if err != nil || !owned["aaaaaaaaaaaaaaaaaaaa"] || budget != 34 {
		t.Fatalf("current manifest not loaded: owned=%v budget=%d err=%v", owned, budget, err)
	}
}

func TestR804PreviousMeshCandidateIDDoesNotBlockNextMeshRound(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	manifest := map[string]any{"schema_version": 3, "project": "P", "fingerprint": "F", "controls": []any{map[string]any{
		"candidate_id": "aaaaaaaaaaaaaaaaaaaa", "name": "AutoMesh_QR_0123456789ab", "object_id": 101,
		"coordinate_system": "AutoMesh_QR_CS_0123456789ab", "cs_object_id": 201,
		"body_id": 1, "centre_mm": []float64{1, 2, 3}, "radius_mm": 2.0, "size_mm": 1.0, "cell_budget_proxy": 34,
	}}}
	if err := writeJSON(w.path("warning_refinement_managed.json"), manifest); err != nil {
		t.Fatal(err)
	}
	s := detailedWarningSnapshot("P", "F")
	owned, budget, err := w.warningOwnedCandidates(s)
	if err != nil || !owned["aaaaaaaaaaaaaaaaaaaa"] || budget != 34 {
		t.Fatalf("history should be counted: %v %d %v", owned, budget, err)
	}
	peak := 0.96
	plan := WarningPlan{Clusters: []WarningHotspot{{ID: "bbbbbbbbbbbbbbbbbbbb", SkewMax: &peak, BodyID: 1, CentreMM: []float64{50, 2, 3}, RadiusMM: 2, SizeMM: 1}}}
	if err := w.reconcileReviewLegacyPublished(s, plan, owned); err != nil {
		t.Fatalf("old candidate ID must be retained as prior-round history: %v", err)
	}
	if owned["bbbbbbbbbbbbbbbbbbbb"] {
		t.Fatal("new mesh candidate was incorrectly marked as already published")
	}
}

func TestR804StaleManifestCanUseFullyVerifiedLiveControlsForIncrementalBudget(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	writeLegacyWarningManifest(t, w, "old-project", "old-geometry")
	s := detailedWarningSnapshot("current-project", "current-geometry")
	owned, budget, err := w.warningOwnedCandidates(s)
	if err != nil || len(owned) != 0 || budget != 34 {
		t.Fatalf("live ACT control should provide a verified adoption budget: %v %d %v", owned, budget, err)
	}
	peak := 0.97
	plan := WarningPlan{Clusters: []WarningHotspot{{ID: "bbbbbbbbbbbbbbbbbbbb", SkewMax: &peak, BodyID: 1, CentreMM: []float64{40, 2, 3}, RadiusMM: 2, SizeMM: 1}}}
	if err := w.reconcileReviewLegacyPublished(s, plan, owned); err != nil {
		t.Fatalf("fully readable live controls should not force read-only mode: %v", err)
	}
}

func TestR803ViewButtonUsesValidatedPreviewNotHTMLExistence(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	s := &Snapshot{Project: "P", Fingerprint: "F"}
	peak := 0.97
	plan := WarningPlan{
		JobID: "job-123", SchemaVersion: 1, Status: "ready", PlanID: "plan-12345678901234567890",
		Project: "P", Fingerprint: "F", WarningCount: 4, CoveredCount: 4,
		UncoveredCount: 0, MeshElements: 3803875,
		Clusters: []WarningHotspot{{
			ID: "aaaaaaaaaaaaaaaaaaaa", SkewMax: &peak, SkewVerified: true,
			BodyID: 1, CentreMM: []float64{1, 2, 3}, RadiusMM: 2, SizeMM: 1,
			ElementIDs: []int{101, 102, 103, 104}, Proxy: 34,
		}},
	}
	if err := writeJSON(w.path("warning_refinement_plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.warningPreviewReceiptPath(), WarningPreviewReceipt{
		JobID: "job-123", PlanID: plan.PlanID, Project: "P", Fingerprint: "F",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.Dir, "warning_refinement_plan.html")); !os.IsNotExist(err) {
		t.Fatalf("test precondition: HTML should not exist yet, got %v", err)
	}
	if !w.hasCurrentWarningReport(s) {
		t.Fatal("view action should be enabled from a valid current preview receipt before HTML is generated")
	}
	if _, err := w.SaveWarningPlanHTML(s, plan.PlanID); err != nil {
		t.Fatalf("view action should generate the HTML on demand: %v", err)
	}
}
