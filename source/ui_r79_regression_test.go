package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestR79TenMillionElementsStillMatchCurrentMarker(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	s := &Snapshot{Project: "E:/ALD/project.wbpj", Fingerprint: "meshhash", Mesh: map[string]any{"Elements": float64(10055480)}}
	marker := map[string]any{"status": "valid", "project": s.Project, "fingerprint": s.Fingerprint,
		"mesh_count": 10055480, "worksheet_count": 297, "selection_source": "named_selection_worksheet_skewness_range",
		"range_min": 0.90, "range_max": 1.00,
		"groups": []any{map[string]any{"name": "AutoMesh_Skew_900_1000", "object_id": 132,
			"ids": makeRange(1, 297)}}}
	if err := writeJSON(w.path("warning_refinement_marker.json"), marker); err != nil {
		t.Fatal(err)
	}
	if !w.hasCurrentMarker(s) {
		t.Fatal("valid 10,055,480-element mesh marker was rejected")
	}
	if _, err := w.SaveSkewMarkerHTML(s); err != nil {
		t.Fatalf("report should accept integral float64: %v", err)
	}
	s.Mesh["Elements"] = float64(10055481)
	if w.hasCurrentMarker(s) {
		t.Fatal("changed mesh count accepted")
	}
	if _, err := w.SaveSkewMarkerHTML(s); err == nil {
		t.Fatal("report accepted old mesh count")
	}
	s.Mesh["Elements"] = float64(10055480)
	s.Fingerprint = "changed"
	if w.hasCurrentMarker(s) {
		t.Fatal("changed geometry marker accepted")
	}
}
func makeRange(start, count int) []int {
	out := make([]int, count)
	for i := range out {
		out[i] = start + i
	}
	return out
}

func TestR79RejectMalformedMarkerAndInvalidCounts(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	s := &Snapshot{Project: "p", Fingerprint: "f", Mesh: map[string]any{"Elements": float64(10055480)}}
	if w.hasCurrentMarker(s) {
		t.Fatal("missing marker accepted")
	}
	if err := os.WriteFile(w.path("warning_refinement_marker.json"), []byte("{partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if w.hasCurrentMarker(s) {
		t.Fatal("truncated marker accepted")
	}
	for _, v := range []any{nil, float64(1.25), float64(-4), "1.0", json.Number("1.1"), "bad"} {
		if _, err := meshCellCount(v); err == nil {
			t.Errorf("unexpectedly accepted invalid count %#v", v)
		}
	}
	for _, v := range []any{float64(10055480), "10055480", json.Number("10055480"), int(10055480)} {
		if got, err := meshCellCount(v); err != nil || got != 10055480 {
			t.Errorf("rejected valid count %#v: %d %v", v, got, err)
		}
	}
}

func TestR79SkewRangeValidationAndButtonGating(t *testing.T) {
	for _, pair := range [][2]string{{"0.90", "1.00"}, {"0", "1"}, {" 0.95 ", "0.96"}} {
		if !validSkewRange(pair[0], pair[1]) {
			t.Errorf("rejected %q", pair)
		}
	}
	for _, pair := range [][2]string{{"", "1"}, {"0.95", "0.90"}, {"0.99", "0.99"}, {"-0.1", "1"}, {"0.90", "1.1"}, {"NaN", "1"}, {"0.9", "Inf"}} {
		if validSkewRange(pair[0], pair[1]) {
			t.Errorf("accepted bad interval %q", pair)
		}
	}
	preScan := workflowAvailability{Ready: true, Connected: true, Ack: true, Selected: true}
	if preScan.Allow("qualitycheck") || preScan.Allow("mark") || preScan.Allow("diagnose") {
		t.Fatal("quality operations must wait for a verified geometry scan")
	}
	s := workflowAvailability{Ready: true, Connected: true, Ack: true, Selected: true, Scanned: true}
	if !s.Allow("qualitycheck") {
		t.Fatal("quality check should be enabled after a verified scan")
	}
	if s.Allow("markhighskew") {
		t.Fatal("invalid range allowed")
	}
	s.SkewRangeValid = true
	if !s.Allow("markhighskew") {
		t.Fatal("valid range should enable action")
	}
}

func TestR79FocusedControlScrollClampedToViewport(t *testing.T) {
	for _, sample := range []struct {
		box        uiBox
		page, w, h int
	}{
		{uiBox{X: 960, Y: 633, W: 170, H: 34}, 3, 940, 630},
		{uiBox{X: 250, Y: 483, W: 170, H: 34}, 3, 1240, 730},
		{uiBox{X: 834, Y: 649, W: 205, H: 34}, 2, 940, 630},
	} {
		x, y := scrollToReveal(sample.box, sample.page, sample.w, sample.h, 0, 0)
		if x < 0 || x > scrollLimitX(sample.w) || y < 0 || y > scrollLimit(sample.page, sample.h) {
			t.Fatalf("invalid offsets %d/%d", x, y)
		}
		if sample.box.X+sample.box.W-x > sample.w-18 && x < scrollLimitX(sample.w) {
			t.Fatal("focus not scrolled far enough")
		}
	}
	if !strings.Contains(taskDisplayName("warning_refine_publish"), "Sizing") {
		t.Fatal("changed task naming unexpectedly")
	}
}
