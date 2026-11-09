package main

import (
	"os"
	"strings"
	"testing"
)

func TestElementQualityMarkerUsesCustomIntervalAndVerifiedScan(t *testing.T) {
	for _, rule := range []int{0, 1, 2, 3, 4, -1} {
		custom := rule == 2 || rule == 3
		if got := isCustomQualityRule(rule); got != custom {
			t.Fatalf("rule %d custom interval: got %t want %t", rule, got, custom)
		}
	}
	if markerRuleAction(3) != "markloweq" || markerRuleAction(2) != "markhighskew" || markerRuleAction(4) != "" {
		t.Fatal("marker rule action mapping is incomplete")
	}
	ready := workflowAvailability{Ready: true, Connected: true, Ack: true, Selected: true, Scanned: true, SkewRangeValid: true}
	if !ready.Allow(markerRuleAction(3)) || !ready.Allow(markerRuleAction(2)) {
		t.Fatal("both interval metrics should be enabled with a valid interval and scan")
	}
	checks := []struct {
		name       string
		mutate     func(*workflowAvailability)
		wantReason string
	}{
		{"geometry_not_scanned", func(v *workflowAvailability) { v.Scanned = false }, "完整几何扫描"},
		{"invalid_interval", func(v *workflowAvailability) { v.SkewRangeValid = false }, "范围"},
		{"busy", func(v *workflowAvailability) { v.Busy = true }, "任务运行"},
	}
	for _, check := range checks {
		v := ready
		check.mutate(&v)
		if v.Allow(markerRuleAction(3)) {
			t.Errorf("Element Quality marker must remain disabled: %s", check.name)
		}
		if reason := v.DenialReason(markerRuleAction(3)); !strings.Contains(reason, check.wantReason) {
			t.Errorf("%s: missing user-facing denial reason, got %q", check.name, reason)
		}
	}
}

func TestElementQualityNativeSelectorHasEventAndLayoutBindings(t *testing.T) {
	ui, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	states, err := os.ReadFile("ui_action_state_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"add(a.ctl(\"markrule\"), \"自定义 Element Quality 区间\")",
		"ui.onMarkerRuleChanged()",
		"isCustomQualityRule(next)",
		"a.eqRangeDraft = [2]string{\"0.00\", \"0.10\"}",
		"a.run(\"mark_low_element_quality\")",
	} {
		if !strings.Contains(string(ui), needle) {
			t.Errorf("missing native UI contract: %q", needle)
		}
	}
	for _, needle := range []string{
		"markerRuleAction(markRule)",
		"available.Allow(markAction)",
		"isCustomQualityRule(markRule)",
	} {
		if !strings.Contains(string(states), needle) {
			t.Errorf("missing button state contract: %q", needle)
		}
	}
}
