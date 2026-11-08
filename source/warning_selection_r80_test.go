package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func make200CandidatePlan() WarningPlan {
	p := WarningPlan{SchemaVersion: 1, Status: "ready", PlanID: "plan-200", Project: "E:/ALD/one", Fingerprint: "geometry-stable", MeshElements: 10055480,
		WarningCount: 200, CoveredCount: 200, Clusters: make([]WarningHotspot, 0, 200)}
	for i := 0; i < 200; i++ {
		peak := 0.9 + float64(i+1)*.0005
		p.Clusters = append(p.Clusters, WarningHotspot{ID: fmtCandidateID(i), SkewMax: &peak, SkewVerified: true,
			BodyID: 1, RadiusMM: 1.5, SizeMM: .5, Proxy: 114, WaferGap: "manual_review_required", CentreMM: []float64{float64(i) * 30, 0, 0}, ElementIDs: []int{10000 + i}})
	}
	return p
}
func fmtCandidateID(i int) string {
	// Deliberately 20 lowercase hex chars, same format as the Python worker.
	b := []byte("00000000000000000000")
	for j := 19; j >= 0 && i > 0; j-- {
		b[j] = "0123456789abcdef"[i%16]
		i /= 16
	}
	return string(b)
}

func TestR80UserSelects15Of200WithoutAutoselect(t *testing.T) {
	p := make200CandidatePlan()
	if got, err := validateWarningSelection(p, nil, nil, 0); err == nil || got.SchemaVersion != 0 {
		t.Fatal("no checkboxes selected must fail")
	}
	selected := make([]string, 15)
	for i := 0; i < 15; i++ {
		selected[i] = p.Clusters[199-i].ID
	}
	first, err := validateWarningSelection(p, selected, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.CandidateIDs) != 15 || first.SelectedBudget != 1710 {
		t.Fatalf("bad first receipt %+v", first)
	}
	published := map[string]bool{}
	for _, id := range selected {
		published[id] = true
	}
	again, err := validateWarningSelection(p, selected, published, first.SelectedBudget)
	if err == nil || !strings.Contains(err.Error(), "已发布") || again.SchemaVersion != 0 {
		t.Fatalf("duplicate Sizing not blocked: %v", err)
	}
	next := make([]string, 15)
	for i := 0; i < 15; i++ {
		next[i] = p.Clusters[184-i].ID
	}
	second, err := validateWarningSelection(p, next, published, first.SelectedBudget)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.CandidateIDs) != 15 || second.SelectedBudget != 3420 {
		t.Fatalf("bad second receipt %+v", second)
	}
	// Simulate 32 GB budget protection before ACT task submission.
	if _, err = validateWarningSelection(p, next, published, 119000); err == nil {
		t.Fatal("cumulative budget oversubscribed")
	}
}

func TestR80SkewMaxRenderedAsActualNumberOrUnknown(t *testing.T) {
	d := t.TempDir()
	w := Workspace{Dir: d}
	s := &Snapshot{Project: "P", Fingerprint: "F"}
	peak := .98256
	p := WarningPlan{SchemaVersion: 1, Status: "ready", PlanID: "abc", Project: s.Project, Fingerprint: s.Fingerprint,
		MeshElements: 10055480, WarningCount: 2, CoveredCount: 2, Clusters: []WarningHotspot{
			{ID: "aaaaaaaaaaaaaaaaaaaa", SkewMax: &peak, SkewVerified: true, BodyID: 1, CentreMM: []float64{1, 2, 3}, RadiusMM: 1.5, SizeMM: .5, ElementIDs: []int{100}, Proxy: 114},
			{ID: "bbbbbbbbbbbbbbbbbbbb", SkewMax: nil, SkewVerified: false, BodyID: 1, CentreMM: []float64{32, 2, 3}, RadiusMM: 1.5, SizeMM: .5, ElementIDs: []int{101}, Proxy: 114},
		}}
	b, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(d, "warning_refinement_plan.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	path, err := w.SaveWarningPlanHTML(s, "abc")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "0.98256") || !strings.Contains(string(doc), "未取得") {
		t.Fatalf("skew display incorrect: %s", string(doc)[max(0, strings.Index(string(doc), "实测最大偏度")-100):])
	}
	if strings.Contains(string(doc), "%!f") {
		t.Fatal("invalid fmt pointer to quality value")
	}
}

func TestR80OverlapAndNearDuplicateReview(t *testing.T) {
	p := make200CandidatePlan()
	// Distinct IDs with near-identical centers on one Body must be rejected.
	p.Clusters[1].CentreMM = []float64{.1, 0, 0}
	ids := []string{p.Clusters[0].ID, p.Clusters[1].ID}
	if _, err := validateWarningSelection(p, ids, nil, 0); err == nil || !strings.Contains(err.Error(), "重合") {
		t.Fatalf("near duplicate not blocked: %v", err)
	}
	p.Clusters[1].CentreMM = []float64{2.9, 0, 0}
	overlapping, err := reviewedSphereOverlap(p, ids, nil)
	if err != nil || overlapping != 1 {
		t.Fatalf("ordinary intersecting spheres should be reported, not blocked: overlap=%d err=%v", overlapping, err)
	}
	if _, err = validateWarningSelection(p, ids, nil, 0); err != nil {
		t.Fatal("ordinary overlap should be manually reviewable:", err)
	}
}

func TestR80PublishedControlGeometryMismatch(t *testing.T) {
	p := make200CandidatePlan()
	c := p.Clusters[0]
	if !candidateMatchesOwned(c, c.BodyID, c.CentreMM, c.RadiusMM, c.SizeMM) {
		t.Fatal("matching previous publication rejected")
	}
	if candidateMatchesOwned(c, c.BodyID, []float64{999, 0, 0}, c.RadiusMM, c.SizeMM) {
		t.Fatal("stale old sphere accepted")
	}
}
