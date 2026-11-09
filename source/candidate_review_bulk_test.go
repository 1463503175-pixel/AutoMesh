package main

import (
	"strings"
	"testing"
)

func TestReviewBulkGroupsUseVerifiedSkew(t *testing.T) {
	p := make200CandidatePlan()
	p.Clusters = p.Clusters[:4]
	values := []float64{0.95, 0.9499, 0.90, 0.89}
	for i := range values {
		p.Clusters[i].SkewMax = &values[i]
	}
	p.Clusters[3].SkewVerified = false
	for _, tc := range []struct {
		group reviewSelectionGroup
		want  int
	}{
		{reviewGroupAll, 4}, {reviewGroupSevere, 1}, {reviewGroupWarning, 2}, {reviewGroupOther, 1},
	} {
		got := selectReviewBulk(p, tc.group, nil, nil, 0)
		if len(got.IDs) != tc.want {
			t.Errorf("group %s: got %d, want %d", tc.group, len(got.IDs), tc.want)
		}
	}
}

func TestReviewBulkSkipsPublishedHistoricalNearDuplicateAndBudget(t *testing.T) {
	p := make200CandidatePlan()
	p.Clusters = p.Clusters[:5]
	published := map[string]bool{p.Clusters[0].ID: true}
	old := []warningControlSnapshot{{BodyID: p.Clusters[1].BodyID, CentreMM: p.Clusters[1].CentreMM, RadiusMM: p.Clusters[1].RadiusMM}}
	// Candidate 3 almost coincides with candidate 2. Cumulative budget
	// is advisory so safe candidates are not silently discarded.
	p.Clusters[3].CentreMM = []float64{60.01, 0, 0}
	got := selectReviewBulk(p, reviewGroupAll, published, old, 119800)
	if len(got.IDs) != 2 || got.IDs[0] != p.Clusters[2].ID || got.IDs[1] != p.Clusters[4].ID || got.Budget != 120028 || got.Skipped != 3 {
		t.Fatalf("unsafe auto-selection: %+v", got)
	}
	if got.BudgetStops != 0 {
		t.Fatalf("budget proxy must not silently skip safe candidates: %+v", got)
	}
}

func TestReviewBulkNeverPublishesOrInventsRisk(t *testing.T) {
	p := make200CandidatePlan()
	p.Clusters = p.Clusters[:1]
	p.Clusters[0].SkewMax = nil
	p.Clusters[0].SkewVerified = false
	if got := selectReviewBulk(p, reviewGroupSevere, nil, nil, 0); len(got.IDs) != 0 {
		t.Fatal("unknown skew must not be severe")
	}
	if got := selectReviewBulk(p, reviewGroupOther, nil, nil, 0); len(got.IDs) != 1 {
		t.Fatal("unknown skew separately grouped")
	}
}

func TestReviewPublishGateExplainsEveryBlockingCondition(t *testing.T) {
	base := reviewPublishGate{Selected: 1, Budget: 1100, DetailsOpened: true, RiskConfirmed: true}
	if reason := base.reason(); reason != "" {
		t.Fatalf("valid selection blocked: %s", reason)
	}
	cases := []struct {
		update   func(*reviewPublishGate)
		contains string
	}{
		{func(g *reviewPublishGate) { g.Busy = true }, "ACT"},
		{func(g *reviewPublishGate) { g.HistoricalLock = "未匹配" }, "未匹配"},
		{func(g *reviewPublishGate) { g.Selected = 0 }, "没有勾选"},
		{func(g *reviewPublishGate) { g.NearDuplicate = true }, "历史 Sizing"},
		{func(g *reviewPublishGate) { g.OverlapError = true }, "影响域冲突"},
		{func(g *reviewPublishGate) { g.DetailsOpened = false }, "查看全部影响域"},
		{func(g *reviewPublishGate) { g.RiskConfirmed = false }, "我已核查"},
	}
	above := base
	above.Budget = 120217
	if reason := above.reason(); reason != "" {
		t.Fatalf("advisory proxy must not disable button: %s", reason)
	}
	for i, tc := range cases {
		g := base
		tc.update(&g)
		if got := g.reason(); !strings.Contains(got, tc.contains) {
			t.Errorf("case %d: %s", i, got)
		}
	}
}
