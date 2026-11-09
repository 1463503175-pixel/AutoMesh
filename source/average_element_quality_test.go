package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func eqFloat(v float64) *float64 { return &v }

func TestElementQualityMeanReceiptIsNotInferred(t *testing.T) {
	summary := map[string]any{
		"mesh_elements": 500,
		"quality": map[string]any{
			"skewness":        map[string]any{"min": 0.1, "mean": 0.2, "max": 0.3, "source": "ACT Skewness"},
			"element_quality": map[string]any{"min": 0.05, "mean": 0.62, "max": 0.96, "source": "ACT MeshMetricType.ElementQuality"},
		},
	}
	payload, _ := json.Marshal(summary)
	q, ok := parseQualitySnapshot(Result{JobID: strings.Repeat("a", 32), Phase: "quality_check", Status: "ok", Summary: payload}, time.Now())
	if !ok || q.EQMean == nil || *q.EQMean != 0.62 || q.EQSource != "ACT MeshMetricType.ElementQuality" || !q.HasElementQuality {
		t.Fatalf("native mean Element Quality not preserved: %+v", q)
	}
	delete(summary["quality"].(map[string]any), "element_quality")
	payload, _ = json.Marshal(summary)
	q, ok = parseQualitySnapshot(Result{JobID: strings.Repeat("b", 32), Phase: "quality_check", Status: "ok", Summary: payload}, time.Now())
	if !ok || q.EQMean != nil || q.HasElementQuality {
		t.Fatal("missing Element Quality must stay missing; skewness must not be converted")
	}
}

func TestElementQualityMeanReportReplacesCFDJudgmentCard(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	summary := map[string]any{"mesh_elements": 300, "quality_gate": "review_required", "quality": map[string]any{
		"element_quality": map[string]any{"min": 0.08, "max": 0.98, "mean": 0.64, "source": "ACT MeshMetricType.ElementQuality"},
	}}
	raw, _ := json.Marshal(summary)
	path, err := w.SaveHTMLReport(nil, Result{Phase: "quality_check", Status: "ok", Summary: raw})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"平均单元质量（Element Quality）", "0.64", "0.98", "辅助诊断状态：review_required", "不代表 Fluent 验收"} {
		if !strings.Contains(string(data), text) {
			t.Errorf("missing quality field %q", text)
		}
	}
	if strings.Contains(string(data), `<div class="k">CFD 质量判断</div>`) {
		t.Fatal("old CFD judgment card still occupies mean quality position")
	}
}

func TestElementQualityMeanTrendComparisonAndSourceProtection(t *testing.T) {
	history := reportFixture()
	history[0].EQMin, history[0].EQMean, history[0].EQMax = eqFloat(0.1), eqFloat(0.51), eqFloat(0.96)
	history[1].EQMin, history[1].EQMean, history[1].EQMax = eqFloat(0.12), eqFloat(0.68), eqFloat(0.98)
	history[0].EQSource, history[1].EQSource = "ACT MeshMetricType.ElementQuality", "ACT MeshMetricType.ElementQuality"
	points := CompatibleQualityTrend(history, history[1])
	if len(points) != 2 || points[0].ElementQualityAverage == nil || *points[0].ElementQualityAverage != 0.51 || points[1].ElementQualityAverage == nil || *points[1].ElementQualityAverage != 0.68 {
		t.Fatal("mean Element Quality history trend unavailable")
	}
	html, err := BuildQualityAnalysisReport(history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `<td>平均单元质量（Element Quality）</td><td>0.51000</td><td>0.68000</td><td>&#43;0.17000</td>`) || !strings.Contains(html, `<th>平均单元质量</th>`) {
		t.Fatal("mean Element Quality missing from comparison and trend")
	}
	history[0].EQSource = "other metric source"
	html, err = BuildQualityAnalysisReport(history, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `<td>平均单元质量（Element Quality）</td><td>0.51000</td><td>0.68000</td><td>不可比较/未取得</td>`) {
		t.Fatal("different metric sources falsely treated as comparable")
	}
}

func TestElementQualityMeanCSVColumnAlignment(t *testing.T) {
	history := reportFixture()
	history[0].EQMin, history[0].EQMean, history[0].EQMax = eqFloat(0.10), eqFloat(0.48), eqFloat(0.97)
	history[0].EQSource = "ACT ElementQuality"
	history[0].HasElementQuality = true
	rows := buildQualityHistoryCSVRows(history)
	if len(rows) != 3 {
		t.Fatalf("wrong csv row count: %d", len(rows))
	}
	indices := map[string]int{}
	for i, col := range rows[0] {
		indices[col] = i
	}
	for i, row := range rows[1:] {
		if len(row) != len(rows[0]) {
			t.Fatalf("CSV row %d has %d fields vs header %d", i, len(row), len(rows[0]))
		}
	}
	if rows[1][indices["eq_mean"]] != "0.4800" || rows[1][indices["eq_source"]] != "ACT ElementQuality" || rows[1][indices["has_element_quality"]] != "true" {
		t.Fatalf("mean Element Quality CSV mapping incorrect: %v", rows[1])
	}
	if rows[2][indices["eq_mean"]] != "未取得" {
		t.Fatal("missing Element Quality incorrectly written as zero")
	}
}
