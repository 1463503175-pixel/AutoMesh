package main

import (
	"strings"
	"testing"
)

func TestEQReviewGroupsOnlyVerifiedValues(t *testing.T) {
	p := make200CandidatePlan()
	p.QualityMetric = "ElementQuality"
	p.Clusters = p.Clusters[:4]
	values := []float64{0.01, 0.05, 0.0999, 0.12}
	for i := range p.Clusters {
		p.Clusters[i].EQMin = &values[i]
		p.Clusters[i].EQVerified = true
	}
	p.Clusters[3].EQVerified = false
	cases := []struct {
		group reviewSelectionGroup
		count int
	}{
		{reviewGroupAll, 4}, {reviewGroupSevere, 1}, {reviewGroupWarning, 2}, {reviewGroupOther, 1},
	}
	for _, c := range cases {
		got := selectReviewBulk(p, c.group, nil, nil, 0)
		if len(got.IDs) != c.count {
			t.Fatalf("EQ group %s: %d != %d", c.group, len(got.IDs), c.count)
		}
	}
	p.Clusters[0].EQMin = nil
	p.Clusters[0].EQVerified = false
	if candidateInMetricReviewGroup(p.Clusters[0], reviewGroupSevere, p.QualityMetric) {
		t.Fatal("unknown Element Quality falsely assigned to severe group")
	}
}

func TestBudgetDiagnosticsExhaustedAndOverlapReadOnly(t *testing.T) {
	c := WarningHotspot{ID: "12345678901234567890", BodyID: 7, CentreMM: []float64{0, 0, 0}, RadiusMM: 5, SizeMM: 1, Proxy: 800}
	old := []warningControlSnapshot{{Name: "AutoMesh_QR_old", BodyID: 7, CentreMM: []float64{0, 0, 0}, RadiusMM: 6, SizeMM: 1, Proxy: 119400}}
	plan := WarningPlan{QualityMetric: "ElementQuality", Clusters: []WarningHotspot{c}}
	before := old[0]
	v := computeBudgetDiagnostic(plan, old, true)
	if v.Historical != 119400 || v.Remaining != 600 || v.MinimumCandidate != 800 {
		t.Fatalf("bad budget analysis: %+v", v)
	}
	if !strings.Contains(v.Rows[0].BudgetStatus, "近重合禁止") || !strings.Contains(v.Rows[0].Relation, "近重合") {
		t.Fatalf("bad overlap diagnosis: %+v", v.Rows[0])
	}
	plan.Clusters[0].CentreMM = []float64{50, 0, 0}
	withoutOverlap := computeBudgetDiagnostic(plan, old, true)
	if !strings.Contains(withoutOverlap.Rows[0].BudgetStatus, "超建议预算") {
		t.Fatalf("budget exhaustion not explained: %+v", withoutOverlap.Rows[0])
	}
	if old[0].Name != before.Name || old[0].Proxy != before.Proxy {
		t.Fatal("read-only report modified historical sizing")
	}
	if res := computeBudgetDiagnostic(plan, nil, false); res.Known || !strings.Contains(res.Rows[0].BudgetStatus, "未核验") {
		t.Fatalf("unsafe unknown budget: %+v", res)
	}
}

func TestBudgetDiagnosticsFullyCoveredAndDifferentBody(t *testing.T) {
	c := WarningHotspot{BodyID: 7, CentreMM: []float64{4, 0, 0}, RadiusMM: 1, Proxy: 200}
	old := []warningControlSnapshot{{Name: "old", BodyID: 7, CentreMM: []float64{0, 0, 0}, RadiusMM: 10, SizeMM: 1, Proxy: 400}}
	if relation := candidateControlRelation(c, old[0]); !strings.Contains(relation, "完全位于") {
		t.Fatal(relation)
	}
	old[0].BodyID = 8
	if relation := candidateControlRelation(c, old[0]); relation != "" {
		t.Fatalf("cross Body overlap invented: %s", relation)
	}
}
