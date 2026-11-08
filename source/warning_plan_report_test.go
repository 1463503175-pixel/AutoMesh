package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarningPlanReportGrounding(t *testing.T) {
	d := t.TempDir()
	w := Workspace{Dir: d}
	s := &Snapshot{Project: "E:/ALD/instance", Fingerprint: "123"}
	p := WarningPlan{SchemaVersion: 1, Status: "ready", PlanID: "abcdef", Project: s.Project, Fingerprint: s.Fingerprint,
		WarningCount: 3, CoveredCount: 2, UncoveredCount: 1, MeshElements: 9660873, Clusters: []WarningHotspot{{BodyID: 1, CentreMM: []float64{1, 2, 3}, RadiusMM: 3, SizeMM: 1, ElementIDs: []int{101, 102}}}}
	b, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(d, "warning_refinement_plan.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.SaveWarningPlanHTML(s, "wrong-id"); err == nil {
		t.Fatal("accepted wrong plan")
	}
	path, err := w.SaveWarningPlanHTML(s, "abcdef")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := os.ReadFile(path)
	for _, want := range []string{"9,660,873", "101, 102", "Body ID", "球心"} {
		if want == "9,660,873" {
			want = "9660873"
		}
		if !strings.Contains(string(doc), want) {
			t.Errorf("missing %s", want)
		}
	}
}
