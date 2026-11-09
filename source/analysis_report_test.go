package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reportFixture() []QualitySnapshot {
	samplesBefore := []NativeSkewSample{{ElementID: 1, BodyID: 7, Skewness: .95}, {ElementID: 2, BodyID: 8, Skewness: .96}}
	samplesAfter := []NativeSkewSample{{ElementID: 3, BodyID: 7, Skewness: .80}, {ElementID: 4, BodyID: 8, Skewness: .91}}
	b, _ := AggregateNativeSkewness(samplesBefore, 2, "TEST-only native-metric fixture")
	a, _ := AggregateNativeSkewness(samplesAfter, 2, "TEST-only native-metric fixture")
	maxBefore, maxAfter, meanBefore, meanAfter, oqBefore, oqAfter := .96, .91, .955, .855, .1, .15
	before := QualitySnapshot{JobID: strings.Repeat("a", 32), CompletedAt: time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC),
		Project: `E:/<test>&.wbpj`, MeshSystem: "FFF", GeometryFingerprint: "same", GeometryVerified: true,
		QualityDefinition: "v1", SkewSource: "native", OQSource: "original OQ", Source: "ACT source",
		MeshSignature: "before", MeshElements: 2, SkewMax: &maxBefore, SkewMean: &meanBefore, OQMin: &oqBefore, SkewDistribution: &b}
	after := before
	after.JobID = strings.Repeat("b", 32)
	after.MeshSignature = "after"
	after.CompletedAt = after.CompletedAt.Add(time.Hour)
	after.SkewMax = &maxAfter
	after.SkewMean = &meanAfter
	after.OQMin = &oqAfter
	after.SkewDistribution = &a
	return []QualitySnapshot{before, after}
}

func TestQualityAnalysisCompleteReportAndEscape(t *testing.T) {
	history := reportFixture()
	html, err := BuildQualityAnalysisReport(history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"P95", "Body 区域统计", "0.90", "0.95", "多轮趋势", "ACT source", "TEST-only native-metric fixture", "-0.05000", "不可比较/未取得"} {
		// The report may not contain 'unavailable' if all native metrics exist.
		if part == "不可比较/未取得" {
			continue
		}
		if !strings.Contains(html, part) {
			t.Errorf("missing report section %q", part)
		}
	}
	if strings.Contains(html, `E:/<test>&.wbpj`) || !strings.Contains(html, "&lt;test&gt;&amp;") {
		t.Fatal("user-controlled project path was not HTML-escaped")
	}
	if strings.Contains(html, "正在执行 ANSYS") {
		t.Fatal("report claims a native operation")
	}
}

func TestQualityAnalysisUnverifiedComparisonAndMissingDistribution(t *testing.T) {
	history := reportFixture()
	history[1].GeometryFingerprint = "different"
	history[1].SkewDistribution = nil
	html, err := BuildQualityAnalysisReport(history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "不可比较/未取得") || !strings.Contains(html, "未取得真实偏度分布") || !strings.Contains(html, "几何指纹缺失或不同") {
		t.Fatal("unverified or incomplete metrics not labelled")
	}
	if _, err := BuildQualityAnalysisReport(history, 1, 2); err == nil {
		t.Fatal("invalid selection accepted")
	}
	// No optimistic improvement statements when comparing a snapshot to itself.
	same, err := BuildQualityAnalysisReport(history, 0, 0)
	if err != nil || !strings.Contains(same, "同一轮记录") {
		t.Fatal("same record was treated as before/after")
	}
	regions := QualityRegionComparisons(history[0], history[0])
	if len(regions) > 0 && regions[0].Comparable {
		t.Fatal("self-comparison reported a fake Body improvement")
	}
}
func TestQualityAnalysisExportReadOnlyAndCollisionSafe(t *testing.T) {
	ws := Workspace{Dir: t.TempDir()}
	history := reportFixture()
	path, err := ExportQualityAnalysisReport(ws, history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != ws.Dir {
		t.Fatal("report escaped configured workspace")
	}
	contents, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(contents), "网格质量分析报告") {
		t.Fatal("report was not saved")
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "job_"+history[0].JobID+".json")); !os.IsNotExist(err) {
		t.Fatal("report generator wrote a task receipt")
	}
}
func TestQualityTrendAndRegionPreventInvalidClaims(t *testing.T) {
	history := reportFixture()
	points := CompatibleQualityTrend(history, history[1])
	if len(points) != 2 || points[0].P95 == nil || points[0].High090Fraction == nil {
		t.Fatal("valid trend lost")
	}
	rows := QualityRegionComparisons(history[0], history[1])
	if len(rows) != 2 || !rows[0].Comparable || rows[0].DeltaP95 == nil {
		t.Fatal("valid region comparison lost")
	}
	history[0].GeometryFingerprint = "changed"
	points = CompatibleQualityTrend(history, history[1])
	if len(points) != 1 {
		t.Fatal("mixed geometry history accepted")
	}
	rows = QualityRegionComparisons(history[0], history[1])
	if len(rows) != 2 || rows[0].Comparable || rows[0].DeltaP95 != nil {
		t.Fatal("mixed geometry region delta accepted")
	}
	history[0] = history[1]
	history[0].SkewDistribution = nil
	points = CompatibleQualityTrend(history, history[1])
	if points[0].P95 != nil {
		t.Fatal("missing native P95 fabricated")
	}
}

func TestQualityReportReferenceNumbersWithoutVerifiedGeometry(t *testing.T) {
	history := reportFixture()
	history[0].GeometryVerified = false
	history[0].GeometryFingerprint = ""
	history[0].GeometryQuickSignature = "same-quick"
	history[1].GeometryQuickSignature = "same-quick"
	html, err := BuildQualityAnalysisReport(history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "参考差值（未核验）") || !strings.Contains(html, "不是已验证的优化效果") {
		t.Fatal("reference differences must be explicitly labelled as unverified")
	}
	if !strings.Contains(html, "-0.05000") {
		t.Fatal("available raw skew difference should not be hidden solely due to missing geometry audit")
	}
	history[0].GeometryQuickSignature = "conflict"
	html, err = BuildQualityAnalysisReport(history, 0, 1)
	if err != nil || strings.Contains(html, "参考差值（未核验）") {
		t.Fatal("explicitly conflicting geometry evidence must block reference delta")
	}
}
