package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmallFeatureReadOnlyLocalCADHint(t *testing.T) {
	s := &Snapshot{Project: "E:/ALD", Fingerprint: "verified", GeometryUnit: "mm", Faces: []Face{
		{ID: 45, Body: 9, Area: 22, Type: "Plane", Centroid: []float64{6, 1, 0}, BBox: []float64{0, 0, 0, 12, 2, 0}, Edges: []FeatureEdge{{ID: 77, Length: .6}, {ID: 78, Length: 12}}},
		{ID: 46, Body: 99, Area: 22, Type: "Plane", Centroid: []float64{6, 1, 0}, BBox: []float64{0, 0, 0, 12, 2, 0}, Edges: []FeatureEdge{{ID: 79, Length: .3}}},
	}}
	p := WarningPlan{SchemaVersion: 1, PlanID: "checked", Project: s.Project, Fingerprint: s.Fingerprint,
		Status: "ready", WarningCount: 1, CoveredCount: 1, QualityMetric: "ElementQuality", Clusters: []WarningHotspot{{
			ID: "a", BodyID: 9, CentreMM: []float64{11, 1, .2}, RadiusMM: 1.5, SizeMM: .8,
			ElementIDs: []int{501}, Proxy: 28,
		}}}
	got := diagnosticSmallFeatures(s, p)
	if len(got) != 1 || len(got[0].FaceDetails) != 1 {
		t.Fatalf("unexpected feature evidence: %+v", got)
	}
	f := got[0].FaceDetails[0]
	if f.FaceID != 45 || f.EdgeID != 77 || f.ShortestEdgeMM != .6 || !strings.Contains(f.Hint, "短 CAD 边") {
		t.Fatalf("bad CAD evidence: %+v", f)
	}
	// This diagnostic may write an HTML report only; it never publishes a control.
	d := t.TempDir()
	w := Workspace{Dir: d}
	buf, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(d, "warning_refinement_plan.json"), buf, 0600); err != nil {
		t.Fatal(err)
	}
	path, err := w.SaveWarningPlanHTML(s, p.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"小凸起 / 短边", "Face <strong>45</strong>", "Edge <strong>77</strong>", "人工核验", "不减少历史预算"} {
		if !strings.Contains(string(html), value) {
			t.Fatalf("report missing %q", value)
		}
	}
}

func TestSmallFeatureDoesNotInventUnknownScope(t *testing.T) {
	c := WarningHotspot{ID: "c", BodyID: 9, CentreMM: []float64{11, 1, 0}, RadiusMM: 2, SizeMM: .9}
	p := WarningPlan{Clusters: []WarningHotspot{c}}
	cases := []Snapshot{
		{GeometryUnit: "unknown", Faces: []Face{{ID: 45, Body: 9, Centroid: []float64{11, 1, 0}}}},
		{GeometryUnit: "mm", Faces: []Face{{ID: 45, Body: 9, Area: 1}}},
		{GeometryUnit: "mm", Faces: []Face{{ID: 45, Body: 8, Centroid: []float64{11, 1, 0}, BBox: []float64{10, 0, 0, 12, 2, 2}}}},
	}
	for _, s := range cases {
		v := diagnosticSmallFeatures(&s, p)
		if len(v) != 1 || len(v[0].FaceDetails) != 0 {
			t.Fatalf("invented a face mapping: %+v", v)
		}
	}
	// Explicit ACT face link is never silently changed into a publishable scope.
	p.Clusters[0].MappedFaceIDs = []int{45}
	s := Snapshot{GeometryUnit: "mm", Faces: []Face{{ID: 45, Body: 9, Area: 1}}}
	v := diagnosticSmallFeatures(&s, p)
	if len(v[0].FaceDetails) != 1 || !strings.Contains(v[0].FaceDetails[0].Method, "ACT 元素") {
		t.Fatalf("lost direct face reference: %+v", v)
	}
}

func TestSmallFeatureBBoxAmbiguityFailsClosed(t *testing.T) {
	f := Face{BBox: []float64{0, 1, 2, 3, 4, 5}, Centroid: []float64{.5, 2.5, 4}}
	if _, ok := verifiedFeatureBox(f, 1); ok {
		t.Fatal("ambiguous bbox layout must not produce an unverified face match")
	}
	f.Centroid = []float64{2, 2.5, 4}
	if _, ok := verifiedFeatureBox(f, 1); !ok {
		t.Fatal("expected an unambiguous bbox interpretation")
	}
}
