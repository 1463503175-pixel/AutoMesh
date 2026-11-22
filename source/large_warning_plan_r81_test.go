package main

import "testing"

func TestLargeWarningBatchReceiptValidation(t *testing.T) {
	s := &Snapshot{Project: "E:/ALD/One", Fingerprint: "mock-geometry"}
	p := WarningPlan{
		SchemaVersion: 1, Status: "ready", PlanID: "bulk-review",
		Project: s.Project, Fingerprint: s.Fingerprint,
		WarningCount: 11671, CoveredCount: 11200, UncoveredCount: 471,
		AnalysisBatchCount: 12, AnalysisBatchSize: 1024,
		SpatialBatchCount: 12, AnalysisProcessedCount: 11671,
		AnalysisComplete: true,
		Clusters: []WarningHotspot{{ID: "one", BodyID: 1,
			CentreMM: []float64{0, 0, 0}, RadiusMM: 1.5,
			SizeMM: 0.5, ElementIDs: []int{1}}},
	}
	if err := validateWarningDiagnostic(p, s, "bulk-review"); err != nil {
		t.Fatalf("complete 11,671-element plan should be reviewable: %v", err)
	}
	p.AnalysisBatchSize = 5000
	if err := validateWarningDiagnostic(p, s, "bulk-review"); err == nil {
		t.Fatal("a claimed ACT batch >4096 must be rejected")
	}
	p.AnalysisBatchSize = 1024
	p.AnalysisProcessedCount--
	if err := validateWarningDiagnostic(p, s, "bulk-review"); err == nil {
		t.Fatal("missing elements must not pass review")
	}
}
