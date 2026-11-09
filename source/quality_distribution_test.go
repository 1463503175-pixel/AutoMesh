package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestNativeSkewnessStatisticsExactThresholdsAndBody(t *testing.T) {
	// Test-only synthetic values. Never exported as real ANSYS measurements.
	samples := []NativeSkewSample{
		{ElementID: 1, BodyID: 12, Skewness: .2},
		{ElementID: 2, BodyID: 12, Skewness: .9},
		{ElementID: 3, BodyID: 12, Skewness: .90000001},
		{ElementID: 4, BodyID: 13, Skewness: .95},
		{ElementID: 5, BodyID: 13, Skewness: .95000001},
		{ElementID: 6, BodyID: 13, Skewness: 1},
	}
	got, err := AggregateNativeSkewness(samples, 6, "TEST fixture - not ANSYS")
	if err != nil {
		t.Fatal(err)
	}
	if got.Above090 != 4 || got.Above095 != 2 || got.P95 != 1 {
		t.Fatalf("incorrect exact statistics: %+v", got)
	}
	if len(got.BodyStats) != 2 || got.BodyStats[0].Count != 3 || got.BodyStats[1].Count != 3 || got.BodyStats[1].Above095 != 2 {
		t.Fatalf("bad region calculation: %+v", got.BodyStats)
	}
	if err := validateSkewnessDistribution(got, 6); err != nil {
		t.Fatal(err)
	}
	if got.Bins[8].Count != 1 || got.Bins[9].Count != 2 || got.Bins[10].Count != 2 {
		// Other values: 0.2 (bin 1), 0.9 (bin 8), 0.90000001+0.95 (bin 9), 0.95000001+1 (bin 10).
		t.Fatalf("incorrect histogram at limits: %+v", got.Bins)
	}
}

func TestSkewDistributionCannotUsePartialOrInvalidMeasurements(t *testing.T) {
	rows := []NativeSkewSample{{ElementID: 1, BodyID: 1, Skewness: .95}, {ElementID: 2, BodyID: 1, Skewness: .96}}
	if _, err := AggregateNativeSkewness(rows, 3, "native"); err == nil {
		t.Fatal("partial capture accepted")
	}
	if _, err := AggregateNativeSkewness(rows, 2, ""); err == nil {
		t.Fatal("unknown provenance accepted")
	}
	bad := [][]NativeSkewSample{
		{{ElementID: 1, Skewness: .5}, {ElementID: 1, Skewness: .6}},
		{{ElementID: 1, Skewness: -.1}, {ElementID: 2, Skewness: .6}},
		{{ElementID: 1, Skewness: math.NaN()}, {ElementID: 2, Skewness: .6}},
	}
	for _, sample := range bad {
		if _, err := AggregateNativeSkewness(sample, 2, "native"); err == nil {
			t.Fatal("invalid metric accepted")
		}
	}
	valid, err := AggregateNativeSkewness(rows, 2, "TEST fixture")
	if err != nil {
		t.Fatal(err)
	}
	valid.Bins[0].Count++
	if validateSkewnessDistribution(valid, 2) == nil {
		t.Fatal("mismatching totals accepted")
	}
	valid, _ = AggregateNativeSkewness(rows, 2, "TEST fixture")
	valid.BodyStats[0].Count++
	if validateSkewnessDistribution(valid, 2) == nil {
		t.Fatal("partial region validation accepted")
	}
}

func TestSnapshotOnlyAcceptsCompleteNativeSkewDistribution(t *testing.T) {
	id := strings.Repeat("f", 32)
	dist, err := AggregateNativeSkewness([]NativeSkewSample{{ElementID: 1, BodyID: 1, Skewness: .8}, {ElementID: 2, BodyID: 1, Skewness: .97}}, 2, "TEST native export")
	if err != nil {
		t.Fatal(err)
	}
	quality := map[string]any{
		"elements":              2,
		"skewness":              map[string]any{"min": .8, "mean": .885, "max": .97, "source": "ACT MeshMetricType.Skewness"},
		"skewness_distribution": dist,
	}
	summary := map[string]any{"mesh_elements": 2, "quality": quality}
	encoded, _ := json.Marshal(summary)
	receipt := Result{JobID: id, Status: "ok", Phase: "quality_check", Summary: encoded}
	got, ok := parseQualitySnapshot(receipt, time.Now())
	if !ok || got.SkewDistribution == nil || got.SkewDistribution.Above095 != 1 {
		t.Fatalf("valid native distribution lost: %+v", got)
	}
	// Legacy receipt: show unavailable, never create estimated P95/threshold data.
	delete(quality, "skewness_distribution")
	encoded, _ = json.Marshal(summary)
	receipt.Summary = encoded
	got, ok = parseQualitySnapshot(receipt, time.Now())
	if !ok || got.SkewDistribution != nil {
		t.Fatalf("missing native stats fabricated: %+v", got)
	}
	quality["skewness_distribution"] = dist
	summary["mesh_elements"] = 3
	encoded, _ = json.Marshal(summary)
	receipt.Summary = encoded
	got, _ = parseQualitySnapshot(receipt, time.Now())
	if got.SkewDistribution != nil {
		t.Fatal("incomplete mesh coverage accepted")
	}
}

func TestNativeBodyBeforeAfterComparisonRequiresMatchingIdentity(t *testing.T) {
	a := QualitySnapshot{Project: "E:/test.wbpj", MeshSystem: "FFF", GeometryFingerprint: "g1", GeometryVerified: true, QualityDefinition: "v1", SkewSource: "native"}
	b := a
	before, _ := AggregateNativeSkewness([]NativeSkewSample{{ElementID: 1, BodyID: 4, Skewness: .96}}, 1, "native")
	after, _ := AggregateNativeSkewness([]NativeSkewSample{{ElementID: 2, BodyID: 4, Skewness: .80}}, 1, "native")
	a.SkewDistribution = &before
	b.SkewDistribution = &after
	p, q, ok := comparableNativeBodyStats(a, b, 4)
	if !ok || p.Above095 != 1 || q.Above095 != 0 {
		t.Fatalf("invalid Body comparison: %+v %+v %v", p, q, ok)
	}
	b.GeometryFingerprint = "changed"
	if _, _, ok := comparableNativeBodyStats(a, b, 4); ok {
		t.Fatal("different geometry compared")
	}
	b = a
	b.SkewDistribution = &after
	b.SkewDistribution.Source = "different source"
	if _, _, ok := comparableNativeBodyStats(a, b, 4); ok {
		t.Fatal("different metric source compared")
	}
	b = a
	b.SkewDistribution = &after
	if _, _, ok := comparableNativeBodyStats(a, b, 99); ok {
		t.Fatal("missing region treated as zero")
	}
}

func TestDeepNativeQualityUsesOnlyExplicitFlagAndCanonicalPhase(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	normal, e := w.SubmitForSystem("FFF", "quality_check", nil, nil, "", false)
	if e != nil {
		t.Fatal(e)
	}
	if normal.DeepNativeSkewness || normal.Phase != "quality_check" {
		t.Fatalf("unexpected default deep scan: %+v", normal)
	}
	// Test-only lease release: no remote job has been dispatched.
	if e = w.removeTaskLeaseIfOwned(normal.JobID); e != nil {
		t.Fatal(e)
	}
	deep, e := w.SubmitForSystem("FFF", "quality_check_deep", nil, nil, "", false)
	if e != nil {
		t.Fatal(e)
	}
	if !deep.DeepNativeSkewness || deep.Phase != "quality_check" {
		t.Fatalf("deep scan metadata missing: %+v", deep)
	}
}
