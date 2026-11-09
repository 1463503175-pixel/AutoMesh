package main

import "testing"

func TestAnalysisNativeDistributionButtonNoOverlap(t *testing.T) {
	keys := []string{"analysisbase", "analysistarget", "analysisrefresh", "analysisexport", "analysisdeep"}
	for _, width := range []int{1100, 1120, 1280, 1600, 1920} {
		layout := plannedControls(width)
		for i, a := range keys {
			box, ok := layout[a]
			if !ok || box.W <= 0 || box.X+box.W > width {
				t.Fatalf("width %d: invalid %s %+v", width, a, box)
			}
			for _, b := range keys[i+1:] {
				if box.intersects(layout[b]) {
					t.Fatalf("width %d: %s overlaps %s", width, a, b)
				}
			}
		}
	}
	if p, ok := pageControlPage("analysisdeep"); !ok || p != 4 {
		t.Fatal("analysis button not assigned to the Analysis Center page")
	}
}
