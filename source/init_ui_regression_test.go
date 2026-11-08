package main

import "testing"

func TestConnectionInitializationButtonsHaveNoOverlap(t *testing.T) {
	for _, width := range []int{1120, 1240, 1440, 1920} {
		boxes := plannedControls(width)
		keys := []string{"port", "link", "initretry", "taskstatusconn"}
		for i := 0; i < len(keys); i++ {
			a, exists := boxes[keys[i]]
			if !exists || a.W < 120 {
				t.Fatalf("width %d missing or too small %s", width, keys[i])
			}
			for j := i + 1; j < len(keys); j++ {
				if a.intersects(boxes[keys[j]]) {
					t.Fatalf("width %d overlap: %s %s", width, keys[i], keys[j])
				}
			}
			if a.X+a.W > contentX+uiWidth(width)-15 {
				t.Fatalf("width %d outside card: %s", width, keys[i])
			}
		}
	}
}
