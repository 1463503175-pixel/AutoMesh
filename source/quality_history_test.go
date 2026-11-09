package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQualityHistoryUsesPerJobReceiptsAndSorts(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	base := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	makeResult := func(id, completed string, max float64) Result {
		summary, _ := json.Marshal(map[string]any{
			"project": "E:/ALD/test.wbpj", "mesh_system": "FFF", "geometry_fingerprint": "geom-1",
			"geometry_identity_verified": true, "mesh_signature": "mesh-" + id,
			"quality_definition": "automesh-act-quality-v1", "mesh_elements": 1000,
			"quality": map[string]any{"elements": 1000, "nodes": 1200,
				"skewness": map[string]any{"min": 0.01, "mean": 0.2, "max": max, "source": "ACT Skewness"}},
		})
		return Result{JobID: id, Phase: "quality_check", Status: "ok", CompletedAt: completed, Summary: summary}
	}
	ids := []string{strings.Repeat("a", 32), strings.Repeat("b", 32)}
	for i, id := range ids {
		completed := base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339)
		if err := writeJSON(w.path("job_"+id+".json"), makeResult(id, completed, 0.9-float64(i)*0.1)); err != nil {
			t.Fatal(err)
		}
	}
	// A non-quality task must not become a quality snapshot.
	if err := writeJSON(w.path("job_"+strings.Repeat("c", 32)+".json"), Result{JobID: strings.Repeat("c", 32), Phase: "inspect", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	history, err := w.LoadQualityHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(history))
	}
	if history[0].JobID != ids[0] || history[1].JobID != ids[1] {
		t.Fatalf("history order incorrect: %+v", history)
	}
	if history[0].SkewMax == nil || *history[0].SkewMax != 0.9 {
		t.Fatalf("max skew missing: %+v", history[0])
	}
	if ok, _ := qualitySnapshotsComparable(history[0], history[1]); !ok {
		t.Fatal("same geometry/definition should compare")
	}
}

func TestQualityHistoryRejectsDifferentGeometryAndMissingDefinition(t *testing.T) {
	a := QualitySnapshot{Project: "E:/a.wbpj", GeometryFingerprint: "g1", QualityDefinition: "v1"}
	b := a
	b.GeometryFingerprint = "g2"
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("different geometry must not compare")
	}
	b = a
	b.QualityDefinition = ""
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("missing definition must not compare")
	}
}

func TestLoadQualityHistoryDoesNotModifyReceipts(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	id := strings.Repeat("d", 32)
	payload := []byte(`{"job_id":"` + id + `","phase":"quality_check","status":"ok","summary":{"quality":{"skewness":{"min":0.1,"mean":0.2,"max":0.3}}}}`)
	path := filepath.Join(w.Dir, "job_"+id+".json")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err := w.LoadQualityHistory(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("history load modified a task receipt")
	}
}

func TestQualityHistoryUsesExactUnixCompletionTime(t *testing.T) {
	id := strings.Repeat("e", 32)
	summary, _ := json.Marshal(map[string]any{"quality": map[string]any{"skewness": map[string]any{"min": 0.1, "mean": 0.2, "max": 0.3}}})
	want := time.Date(2026, 11, 8, 10, 11, 12, 345000000, time.UTC)
	q, ok := parseQualitySnapshot(Result{JobID: id, Phase: "quality_check", Status: "ok", CompletedAtUnix: float64(want.UnixNano()) / float64(time.Second), Summary: summary}, time.Time{})
	delta := q.CompletedAt.Sub(want)
	if !ok || delta > time.Microsecond || delta < -time.Microsecond {
		t.Fatalf("completion time mismatch: got %v want %v (delta %v)", q.CompletedAt, want, delta)
	}
}

func TestQualitySnapshotsNormalizeWindowsPathSeparators(t *testing.T) {
	a := QualitySnapshot{Project: `E:\ALD\model.wbpj`, MeshSystem: "FFF", GeometryFingerprint: "g1", QualityDefinition: "v1", GeometryVerified: true, SkewSource: "ACT MeshMetricType.Skewness"}
	b := QualitySnapshot{Project: `e:/ald/model.wbpj`, MeshSystem: "fff", GeometryFingerprint: "g1", QualityDefinition: "v1", GeometryVerified: true, SkewSource: "ACT MeshMetricType.Skewness"}
	if ok, note := qualitySnapshotsComparable(a, b); !ok {
		t.Fatalf("same Windows project path should compare: %s", note)
	}
}

func TestQualitySnapshotsRejectDifferentSkewSources(t *testing.T) {
	a := QualitySnapshot{Project: "E:/a.wbpj", MeshSystem: "FFF", GeometryFingerprint: "g1", QualityDefinition: "v1", GeometryVerified: true, SkewSource: "ACT source A"}
	b := a
	b.SkewSource = "ACT source B"
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("different Skewness collection sources must not be compared")
	}
	b = a
	b.SkewSource = ""
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("missing Skewness collection source must not be compared")
	}
}

func TestQualitySnapshotsRejectDifferentMeshSystems(t *testing.T) {
	a := QualitySnapshot{Project: "E:/a.wbpj", MeshSystem: "FFF", GeometryFingerprint: "g1", QualityDefinition: "v1", GeometryVerified: true, SkewSource: "ACT source"}
	b := a
	b.MeshSystem = "GGG"
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("different Mesh systems must not be compared")
	}
	b = a
	b.MeshSystem = ""
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("missing Mesh system identity must not be compared")
	}
}

func TestQualityHistoryNativeDistributionWithLegacyElementCountFallback(t *testing.T) {
	d, err := AggregateNativeSkewness([]NativeSkewSample{{ElementID: 2, BodyID: 7, Skewness: .97}}, 1, "TEST fixture")
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := json.Marshal(map[string]any{"quality": map[string]any{
		"elements":              1,
		"skewness":              map[string]any{"min": .97, "mean": .97, "max": .97, "source": "native source"},
		"skewness_distribution": d,
	}})
	got, ok := parseQualitySnapshot(Result{JobID: strings.Repeat("a", 32), Phase: "quality_check", Status: "ok", Summary: summary}, time.Now())
	if !ok || got.MeshElements != 1 || got.SkewDistribution == nil {
		t.Fatal("legacy total dropped valid exact native distribution")
	}
}

func TestQualitySnapshotsReferenceDeltasKeepStrictGeometryGate(t *testing.T) {
	a := QualitySnapshot{JobID: strings.Repeat("a", 32), Project: `E:\ALD\test.wbpj`, MeshSystem: "FFF",
		GeometryQuickSignature: "quick-1", QualityDefinition: "automesh-act-quality-v1", SkewSource: "ACT native skew"}
	b := a
	b.JobID = strings.Repeat("b", 32)
	b.Project = "e:/ald/test.wbpj"
	if ok, _ := qualitySnapshotsComparable(a, b); ok {
		t.Fatal("unverified geometry must never pass strict comparison")
	}
	if ok, note := qualitySnapshotsReferenceComparable(a, b); !ok || !strings.Contains(note, "参考") {
		t.Fatalf("same coarse geometry should allow labelled reference delta, got %v: %s", ok, note)
	}
	b.GeometryQuickSignature = "quick-2"
	if ok, _ := qualitySnapshotsReferenceComparable(a, b); ok {
		t.Fatal("mismatched coarse geometry cannot allow reference delta")
	}
	b.GeometryQuickSignature = a.GeometryQuickSignature
	b.GeometryFingerprint, a.GeometryFingerprint = "different", "original"
	if ok, _ := qualitySnapshotsReferenceComparable(a, b); ok {
		t.Fatal("known differing full geometry identities cannot allow reference delta")
	}
	b.GeometryFingerprint = a.GeometryFingerprint
	b.SkewSource = "different metric provider"
	if ok, _ := qualitySnapshotsReferenceComparable(a, b); ok {
		t.Fatal("different metric sources cannot allow reference delta")
	}
	b = a
	if ok, _ := qualitySnapshotsReferenceComparable(a, b); ok {
		t.Fatal("same job must never become a comparison")
	}
}
