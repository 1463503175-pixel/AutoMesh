package main

import "testing"

func TestR78ReadOnlyOperationsIgnoreUnrelatedReferenceSize(t *testing.T) {
	for _, phase := range []string{"inspect", "quality_check", "diagnose", "mark_high_skew", "mark_failed", "mark_warning", "warning_refine_preview", "warning_refine_publish", "export", "publish_existing_sizing"} {
		if taskRequiresReferenceSize(phase) {
			t.Errorf("%s must not need Home's reference dimension", phase)
		}
	}
	for _, phase := range []string{"prepare", "publish_names", "generate_reviewed", "auto", "auto_onepass"} {
		if !taskRequiresReferenceSize(phase) {
			t.Errorf("%s must check reference dimension", phase)
		}
	}
}

func TestR78ButtonPolicyMatchesPhasePrerequisites(t *testing.T) {
	blank := workflowAvailability{}
	if blank.Allow("qualitycheck") || blank.Allow("warningpublish") {
		t.Fatal("cannot start a task before connection")
	}
	if !blank.Allow("taskstatus") || !blank.Allow("tasklog") {
		t.Fatal("diagnostic actions must be usable when disconnected")
	}
	connected := workflowAvailability{Ready: true, Connected: true, Selected: true, Ack: true}
	if !connected.Allow("qualitycheck") || connected.Allow("warningplan") || connected.Allow("warningpublish") {
		t.Fatal("read-only available before a quality selection, other actions gated")
	}
	connected.Scanned = true
	connected.HasMarker = true
	if !connected.Allow("finenames") || !connected.Allow("fineglobalreview") {
		t.Fatal("first-time naming and global review must not depend on existing names")
	}
	if connected.Allow("fineprepare") {
		t.Fatal("publishing an empty named list should require a valid scope")
	}
	if !connected.Allow("warningplan") || connected.Allow("warningpublish") {
		t.Fatal("preview available before publish")
	}
	connected.HasPreview = true
	if !connected.Allow("warningpublish") {
		t.Fatal("reviewed plan should enable publication")
	}
	connected.HasNamed = true
	connected.CanGenerate = true
	if !connected.Allow("finegenerate") {
		t.Fatal("verified meshing action should be available")
	}
	connected.HasReport = true
	connected.Busy = true
	for _, action := range []string{"scan", "qualitycheck", "warningplan", "warningpublish", "finegenerate", "markhighskew", "link"} {
		if connected.Allow(action) {
			t.Errorf("%s must be disabled during an active task", action)
		}
	}
	if !connected.Allow("taskstatus") || !connected.Allow("warningview") {
		t.Fatal("read-only status/report must stay accessible during tasks")
	}
}

func TestR78ScrollIsBoundedByPageAndViewport(t *testing.T) {
	if scrollLimit(3, 600) <= 0 {
		t.Fatal("quality page must be scrollable in a short client area")
	}
	if scrollLimit(3, 800) != 0 {
		t.Fatal("large client should not expose a fake scroll area")
	}
	for p := 0; p <= 4; p++ {
		for _, h := range []int{460, 560, 690, 900} {
			max := scrollLimit(p, h)
			if clampScroll(-10, p, h) != 0 || clampScroll(100000, p, h) != max {
				t.Errorf("page %d height %d clamp error", p, h)
			}
		}
	}
}

func TestR78HorizontalScrollMaintainsVirtualContentWidth(t *testing.T) {
	for _, w := range []int{900, 940, 1024, 1100, 1240, 1600} {
		lim := scrollLimitX(w)
		if w < 1100 && lim <= 0 {
			t.Errorf("%d should scroll horizontally", w)
		}
		if w >= 1100 && lim != 0 {
			t.Errorf("%d should not need horizontal scroll", w)
		}
		if clampScrollX(-8, w) != 0 || clampScrollX(9999, w) != lim {
			t.Errorf("bad horizontal clamp %d", w)
		}
		if contentX+uiWidth(w)-lim > w-20 {
			t.Errorf("virtual pane remains clipped at width %d", w)
		}
	}
}
