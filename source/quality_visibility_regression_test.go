package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestQualityReportHidesUnavailableOrthogonalCardWithoutFakingValue(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	summary := map[string]any{"mesh_elements": 1200, "quality": map[string]any{
		"skewness":                      map[string]any{"max": 0.92},
		"element_quality":               map[string]any{"min": 0.12, "mean": 0.75, "max": 0.99},
		"orthogonal_quality_diagnostic": map[string]any{"status": "enum_not_exposed"},
	}}
	b, _ := json.Marshal(summary)
	path, err := w.SaveHTMLReport(nil, Result{Phase: "quality_check", Status: "ok", Summary: b})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, `<div class="k">正交质量最小值</div>`) {
		t.Fatal("unavailable OQ still takes a KPI card")
	}
	if !strings.Contains(s, "正交质量数据未展示") || !strings.Contains(s, "平均单元质量") {
		t.Fatal("diagnostic provenance or verified EQ was dropped")
	}
}

func TestAnalysisReportOmitsFourEntirelyUnavailableMetrics(t *testing.T) {
	history := reportFixture()
	for i := range history {
		history[i].OQMin = nil
		history[i].OQMean = nil
		history[i].OQMax = nil
		history[i].SkewDistribution = nil
	}
	html, err := BuildQualityAnalysisReport(history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, missing := range []string{`<td>最小正交质量</td>`, `<td>P95 偏度（最近秩）</td>`, `<td>偏度严格 &gt; 0.90</td>`, `<th>最小正交</th>`} {
		if strings.Contains(html, missing) {
			t.Fatalf("unavailable metric still visible as a comparison row/column: %s", missing)
		}
	}
	if !strings.Contains(html, "P95、直方图") {
		t.Fatal("missing data should still be explained")
	}
	complete, err := BuildQualityAnalysisReport(reportFixture(), 0, 1)
	if err != nil || !strings.Contains(complete, "P95 偏度") || !strings.Contains(complete, "最小正交质量") {
		t.Fatal("real native data must remain visible")
	}
}
