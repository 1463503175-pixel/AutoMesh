package main

import (
	"os"
	"strings"
	"testing"
)

func TestR74MarkedRangeCanBeViewedBeforeOptimizationPreview(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	snap := &Snapshot{Project: "D:/Engineering/ALD.wbpj", Fingerprint: "fingerprint-1", Mesh: map[string]any{"Elements": 10055480}}
	marker := map[string]any{"status": "valid", "project": snap.Project, "fingerprint": snap.Fingerprint,
		"mesh_count": 10055480, "worksheet_count": 2, "selection_source": "named_selection_worksheet_skewness_range",
		"range_min": 0.90, "range_max": 1.00, "groups": []map[string]any{{"name": "AutoMesh_Skew_900_1000_abcd", "object_id": 123, "ids": []int{11, 94}}}}
	if err := writeJSON(w.path("warning_refinement_marker.json"), marker); err != nil {
		t.Fatal(err)
	}
	file, err := w.SaveSkewMarkerHTML(snap)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "0.900 ～ 1.000") || !strings.Contains(string(raw), "AutoMesh_Skew_900") {
		t.Fatalf("incorrect report: %s", raw)
	}
}
func TestR74OldOrIncorrectMarkerMustNotBePresentedAsCurrent(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	snap := &Snapshot{Project: "D:/Engineering/ALD.wbpj", Fingerprint: "fingerprint-2", Mesh: map[string]any{"Elements": 999}}
	marker := map[string]any{"status": "valid", "project": snap.Project, "fingerprint": "fingerprint-1",
		"mesh_count": 999, "worksheet_count": 1, "selection_source": "named_selection_worksheet_skewness_range",
		"range_min": 0.90, "range_max": 1.00, "groups": []map[string]any{{"name": "AutoMesh_Skew", "object_id": 123, "ids": []int{11}}}}
	if err := writeJSON(w.path("warning_refinement_marker.json"), marker); err != nil {
		t.Fatal(err)
	}
	if _, err := w.SaveSkewMarkerHTML(snap); err == nil {
		t.Fatal("old marker accepted")
	}
	marker["fingerprint"] = snap.Fingerprint
	marker["mesh_count"] = 1234
	if err := writeJSON(w.path("warning_refinement_marker.json"), marker); err != nil {
		t.Fatal(err)
	}
	if _, err := w.SaveSkewMarkerHTML(snap); err == nil {
		t.Fatal("wrong mesh count accepted")
	}
}
