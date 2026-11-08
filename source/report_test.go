package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHTMLReportEscapesProject(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	s := &Snapshot{Project: `E:/<unexpected>&.wbpj`, GeometryUnit: "StandardNMM", Bodies: []Body{{ID: 1}}, Faces: []Face{{ID: 1}}}
	m := map[string]any{"mesh_elements": 12000, "quality_gate": "oq_unverified_manual_review", "quality": map[string]any{"skewness": map[string]any{"max": 0.88}}}
	b, _ := json.Marshal(m)
	p, e := w.SaveHTMLReport(s, Result{Phase: "auto", Status: "ok", Summary: b})
	if e != nil {
		t.Fatal(e)
	}
	html, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(html), `E:/<unexpected>`) {
		t.Fatal("HTML injection not escaped")
	}
	if !strings.Contains(string(html), "oq_unverified_manual_review") {
		t.Fatal("Missing quality state")
	}
}

func TestRiskReportAlwaysShowsExplicitResult(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	summary := map[string]any{"mesh_elements": 2000000,
		"worksheet_diagnostics": map[string]any{"status": "available_read_only",
			"metrics": []any{map[string]any{"name": "Skewness", "failed_count": 89, "warning_count": 127}}},
		"element_probe": map[string]any{"status": "sampled_not_exhaustive",
			"sampling_fraction": 0.003, "checked_element_count": 6000,
			"samples": []any{map[string]any{"element_id": 741, "shape_proxy": 0.09,
				"centroid_mesh_units": []any{1.0, 2.0, 3.0}}}}}
	b, _ := json.Marshal(summary)
	path, err := w.SaveHTMLReport(nil, Result{Phase: "diagnose", Status: "ok", Summary: b})
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []string{"风险单元定位结果", "采样覆盖率", "741", "官方失败计数", "89", "risk_samples.csv", "不是全网格检索"} {
		if !strings.Contains(string(html), req) {
			t.Fatalf("risk result missing %s", req)
		}
	}
	csvData, err := os.ReadFile(w.path("risk_samples.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csvData), "741") {
		t.Fatal("Risk CSV lost element ID")
	}
}

func TestRiskReportNoSamplesIsNotMisrepresentedAsHealthy(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	b, _ := json.Marshal(map[string]any{"mesh_elements": 12000,
		"element_probe": map[string]any{"status": "unavailable", "samples": []any{},
			"error": "MeshData missing"}})
	path, err := w.SaveHTMLReport(nil, Result{Phase: "diagnose", Status: "ok", Summary: b})
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "本次未获得风险单元位置") ||
		!strings.Contains(string(html), "不代表整个网格不存在坏单元") {
		t.Fatal("No-samples report must not claim that mesh is healthy")
	}
}

// A successful named-selection or sizing publication must not destroy the
// last measured mesh-quality result in quality_report.html.
func TestPublishingDoesNotOverwriteQualityReport(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	measured := map[string]any{"mesh_elements": 18123456, "quality": map[string]any{"skewness": map[string]any{"max": 0.836}}}
	mb, _ := json.Marshal(measured)
	qualityPath, err := w.SaveHTMLReport(nil, Result{Phase: "quality_check", Status: "ok", Summary: mb})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(qualityPath)
	ab, _ := json.Marshal(map[string]any{"created_named_count": 4, "generate_mesh_called": false})
	activityPath, err := w.SaveHTMLReport(nil, Result{Phase: "publish_names", Status: "ok", Summary: ab})
	if err != nil {
		t.Fatal(err)
	}
	if qualityPath == activityPath {
		t.Fatal("activity must never overwrite quality report")
	}
	after, _ := os.ReadFile(qualityPath)
	if string(before) != string(after) {
		t.Fatal("quality report overwritten by names publication")
	}
	if !strings.Contains(string(after), "0.836") {
		t.Fatal("measured skewness was lost")
	}
}

func TestWorksheetCountsAndRealMetricDataAreShown(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	summary := map[string]any{
		"mesh_elements":  float64(18098459),
		"quality_source": "ACT 当前网格",
		"quality": map[string]any{"nodes": float64(31000000),
			"skewness": map[string]any{"min": 0.0001, "mean": 0.31, "max": 0.836}},
		"worksheet_diagnostics": map[string]any{"status": "available_read_only", "metrics": []any{
			map[string]any{"name": "Skewness", "failed_count": float64(0), "warning_count": float64(8)},
		}},
	}
	b, _ := json.Marshal(summary)
	p, e := w.SaveHTMLReport(nil, Result{Phase: "quality_check", Status: "ok", Summary: b})
	if e != nil {
		t.Fatal(e)
	}
	content, _ := os.ReadFile(p)
	for _, v := range []string{"Worksheet 失败单元", "Worksheet 警告单元", "0（跨指标合计，非去重单元）", "8（跨指标合计，非去重单元）", "网格节点数", "31000000", "0.31", "ACT 当前网格"} {
		if !strings.Contains(string(content), v) {
			t.Errorf("report lacks actual quality field %s", v)
		}
	}
}
func TestOfficialZeroMarkerReportDoesNotPretendItCreatedGroup(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	b, _ := json.Marshal(map[string]any{"status": "zero_elements", "kind": "failed", "worksheet_count": 0,
		"created_groups": []any{}, "message": "当前阈值下失败数为0；未创建选择组"})
	p, e := w.SaveHTMLReport(nil, Result{Phase: "mark_failed", Status: "ok", Summary: b})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasSuffix(p, "activity_report.html") {
		t.Fatal("marker should not overwrite quality_report.html")
	}
	content, _ := os.ReadFile(p)
	for _, v := range []string{"Worksheet 元素定位操作", "zero_elements", "失败数为0", "新增原生选择组"} {
		if !strings.Contains(string(content), v) {
			t.Errorf("marker result missing %s", v)
		}
	}
}
