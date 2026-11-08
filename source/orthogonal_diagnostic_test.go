package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestR62ReportSaysWhyOrthogonalMissing(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	summary := map[string]any{
		"mesh_elements": 1000,
		"quality": map[string]any{
			"skewness":                      map[string]any{"max": 0.91, "min": 0.01, "mean": 0.2},
			"orthogonal_quality_diagnostic": map[string]any{"status": "enum_not_exposed", "reason": "not exposed"},
		},
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	path, err := w.SaveHTMLReport(nil, Result{Phase: "quality_check", Status: "ok", Summary: data})
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "MeshMetricType 不包含 OrthogonalQuality") {
		t.Fatalf("missing reason in report: %s", path)
	}
	if strings.Contains(string(html), "最小正交质量：0.09") {
		t.Fatal("report must not manufacture OQ from 1-skewness")
	}
}
