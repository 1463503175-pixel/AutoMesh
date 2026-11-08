package main

import (
	"strconv"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQualitySnapshotKeepsOnlyReturnedMetrics(t *testing.T) {
	result := Result{JobID: "job-1", Phase: "quality_check", Status: "ok"}
	summary := map[string]any{
		"mesh_elements": float64(3836723),
		"quality_gate":  "review_required",
		"quality": map[string]any{
			"nodes": float64(912345),
			"skewness": map[string]any{
				"min": 0.01, "max": 0.8943, "mean": 0.2831,
				"source": "ACT MeshMetricType.Skewness",
			},
			"orthogonal_quality_diagnostic": map[string]any{
				"status": "enum_not_exposed",
			},
		},
		"worksheet_diagnostics": map[string]any{
			"metrics": []any{
				map[string]any{"name": "Skewness", "failed_count": float64(0), "warning_count": float64(24)},
			},
		},
	}
	at := time.Date(2026, 11, 8, 10, 30, 0, 0, time.Local)
	snapshot, ok := qualitySnapshotFromSummary(result, summary, "E:/ALD/model.wbpj", "geometry-hash", at)
	if !ok {
		t.Fatal("completed quality result was not captured")
	}
	if snapshot.MeshElements == nil || *snapshot.MeshElements != 3836723 || snapshot.MeshNodes == nil || *snapshot.MeshNodes != 912345 {
		t.Fatalf("unexpected mesh size: elements=%v nodes=%v", snapshot.MeshElements, snapshot.MeshNodes)
	}
	if snapshot.Skewness.Max == nil || *snapshot.Skewness.Max != 0.8943 {
		t.Fatalf("missing measured skewness maximum: %+v", snapshot.Skewness)
	}
	if snapshot.OrthogonalQuality != nil {
		t.Fatal("missing ACT OrthogonalQuality must remain unavailable, not inferred")
	}
	if len(snapshot.Worksheet) != 1 || snapshot.Worksheet[0].Warning == nil || *snapshot.Worksheet[0].Warning != 24 {
		t.Fatalf("worksheet counts not preserved: %+v", snapshot.Worksheet)
	}
}

func TestQualitySnapshotRejectsIncompleteOrFailedResult(t *testing.T) {
	if _, ok := qualitySnapshotFromSummary(Result{JobID: "x", Error: "failed"}, map[string]any{"quality": map[string]any{"skewness": map[string]any{"max": 0.9}}}, "p", "f", time.Now()); ok {
		t.Fatal("failed task must not become a quality snapshot")
	}
	if _, ok := qualitySnapshotFromSummary(Result{JobID: "x"}, map[string]any{"mesh_elements": float64(100)}, "p", "f", time.Now()); ok {
		t.Fatal("mesh element count alone is not a quality measurement")
	}
}

func TestQualitySnapshotsRequireSameProjectAndGeometry(t *testing.T) {
	a := QualitySnapshot{Project: "E:/ALD/model.wbpj", GeometryFingerprint: "same"}
	b := QualitySnapshot{Project: "e:\\ald\\model.wbpj", GeometryFingerprint: "same"}
	if !qualitySnapshotsComparable(a, b) {
		t.Fatal("same Windows project and geometry should be comparable")
	}
	b.GeometryFingerprint = "different"
	if qualitySnapshotsComparable(a, b) {
		t.Fatal("different geometry must not be compared")
	}
	b.GeometryFingerprint = "same"
	b.Project = "E:/ALD/other.wbpj"
	if qualitySnapshotsComparable(a, b) {
		t.Fatal("different project must not be compared")
	}
}

func TestQualityHistoryPersistsDeduplicatesAndCaps(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	base := time.Date(2026, 11, 1, 8, 0, 0, 0, time.Local)
	for i := 0; i < maxQualityHistory+3; i++ {
		item := QualitySnapshot{
			ID: "job-" + strconv.Itoa(i),
			JobID: "job", CapturedAt: base.Add(time.Duration(i)*time.Minute),
			Project: "p", GeometryFingerprint: "g",
		}
		if _, err := w.SaveQualitySnapshot(item); err != nil {
			t.Fatal(err)
		}
	}
	items, err := w.LoadQualityHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != maxQualityHistory {
		t.Fatalf("expected %d history items, got %d", maxQualityHistory, len(items))
	}
	latest := items[len(items)-1]
	latest.QualityGate = "updated"
	items, err = w.SaveQualitySnapshot(latest)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != maxQualityHistory || items[len(items)-1].QualityGate != "updated" {
		t.Fatal("saving an existing ID should replace it without adding a duplicate")
	}
	if _, err := os.Stat(filepath.Join(w.Dir, "quality_history.json")); err != nil {
		t.Fatalf("history file not persisted: %v", err)
	}
}

func TestQualityHistoryRefusesToOverwriteCorruptFile(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	path := w.path("quality_history.json")
	if err := os.WriteFile(path, []byte("{truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := w.SaveQualitySnapshot(QualitySnapshot{ID: "job", CapturedAt: time.Now()})
	if err == nil {
		t.Fatal("corrupt history must not be silently replaced")
	}
}
