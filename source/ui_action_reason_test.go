package main

import (
	"strings"
	"testing"
)

func TestNativeActionDenialReasonsFollowExistingPolicy(t *testing.T) {
	s := workflowAvailability{Ready: true, Connected: true, Ack: true, Selected: true, Scanned: true, SkewRangeValid: true}
	tests := []struct {
		modify           func(*workflowAvailability)
		action, contains string
	}{
		{func(p *workflowAvailability) { p.Busy = true }, "qualitycheck", "任务运行"},
		{func(p *workflowAvailability) { p.Ack = false }, "analysisdeep", "Meshing"},
		{func(p *workflowAvailability) { p.Selected = false }, "qualitycheck", "Mesh"},
		{func(p *workflowAvailability) { p.Scanned = false }, "warningplan", "完整几何扫描"},
		{func(p *workflowAvailability) { p.HasMarker = false }, "warningplan", "标记"},
		{func(p *workflowAvailability) { p.HasPreview = false }, "warningpublish", "审核"},
		{func(p *workflowAvailability) { p.HasNamed = false }, "fineprepare", "Named Selection"},
		{func(p *workflowAvailability) { p.CanGenerate = false }, "finegenerate", "清单"},
	}
	for _, tt := range tests {
		v := s
		tt.modify(&v)
		if v.Allow(tt.action) {
			t.Fatalf("invalid test action %q still enabled", tt.action)
		}
		if got := v.DenialReason(tt.action); !strings.Contains(got, tt.contains) {
			t.Errorf("%s: got %q, expected %q", tt.action, got, tt.contains)
		}
	}
	if got := s.DenialReason("qualitycheck"); got != "可操作" {
		t.Fatal(got)
	}
}
