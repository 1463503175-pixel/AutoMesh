package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorksheetQualityBoundsReadFromNativeReceipt(t *testing.T) {
	source := "ACT Named Selection Worksheet MeshElement Skewness strict GreaterThan"
	sk := SkewThresholdStats{Status: "bounded", Method: "native_worksheet_threshold", Source: source,
		MeshElements: 100, Above090: 11, Above095: 8, P95Lower: .9541015625, P95Upper: .955078125,
		P95Precision: 1.0 / 1024, P95Definition: "nearest_rank", ThresholdCountsExact: true}
	oq := OrthogonalQualityBound{Status: "bounded", Method: "native_worksheet_threshold",
		Source:       "ACT Named Selection Worksheet MeshElement OrthogonalQuality strict LessThan",
		MeshElements: 100, MinLower: .009765625, MinUpper: .0107421875, Precision: 1.0 / 1024}
	summary, _ := json.Marshal(map[string]any{
		"mesh_elements": 100,
		"quality": map[string]any{
			"skewness":                      map[string]any{"min": .1, "max": .99, "mean": .4, "source": "ACT MeshMetricType.Skewness"},
			"skewness_threshold_statistics": sk, "orthogonal_quality_bound": oq,
		},
	})
	result := Result{JobID: strings.Repeat("a", 32), Phase: "quality_check", Status: "ok", Summary: summary}
	snap, ok := parseQualitySnapshot(result, time.Now())
	if !ok || snap.SkewThresholds == nil || snap.OQBound == nil {
		t.Fatalf("valid bounded stats lost: %+v", snap)
	}
	if snap.SkewDistribution != nil || snap.OQMin != nil {
		t.Fatal("bounded values fabricated as exact metrics")
	}
	if snap.SkewThresholds.Above095 != 8 || snap.SkewThresholds.P95Upper-snap.SkewThresholds.P95Lower > .001 {
		t.Fatal("strict threshold count or P95 precision lost")
	}
	csv := buildQualityHistoryCSVRows([]QualitySnapshot{snap})
	if len(csv) != 2 || len(csv[0]) != len(csv[1]) {
		t.Fatalf("CSV row/header shape mismatch: %d %d", len(csv[0]), len(csv[1]))
	}
	sk.Above095 = 12    // exceeds the strictly >.90 count
	sk.P95Upper = .0001 // invalid percentile interval
	summary2, _ := json.Marshal(map[string]any{
		"mesh_elements": 100,
		"quality": map[string]any{
			"skewness":                      map[string]any{"min": .1, "max": .99, "mean": .4, "source": "ACT MeshMetricType.Skewness"},
			"skewness_threshold_statistics": sk,
		},
	})
	result.Summary = summary2
	bad, _ := parseQualitySnapshot(result, time.Now())
	if bad.SkewThresholds != nil {
		t.Fatal("invalid threshold statistics accepted")
	}
}

func TestWorksheetBoundsRejectWrongMeshOrSource(t *testing.T) {
	v := SkewThresholdStats{Status: "bounded", Method: "native_worksheet_threshold", Source: "native ACT",
		MeshElements: 100, Above090: 10, Above095: 2, P95Lower: .42, P95Upper: .4209765625,
		P95Precision: 1.0 / 1024, P95Definition: "nearest_rank", ThresholdCountsExact: true}
	max := .89
	if validateSkewThresholdStats(v, 100, &max) == nil {
		t.Fatal("threshold exceeds native maximum")
	}
	max = .99
	if validateSkewThresholdStats(v, 101, &max) == nil {
		t.Fatal("stale mesh identity accepted")
	}
	v.Source = ""
	if validateSkewThresholdStats(v, 100, &max) == nil {
		t.Fatal("missing source accepted")
	}
}
