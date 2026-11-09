package main

import (
	"os"
	"strings"
	"testing"
)

func TestReviewBulkNativeControlsDoNotOverlap(t *testing.T) {
	keys := []string{"reviewselectall", "reviewselectsevere", "reviewselectwarning", "reviewselectother", "reviewclear", "reviewlist", "reviewrisk", "reviewback", "reviewdetails", "reviewreason", "reviewconfirm"}
	for _, width := range []int{1100, 1120, 1280, 1600, 1920} {
		boxes := plannedControls(width)
		for i, name := range keys {
			box := boxes[name]
			if box.W <= 0 || box.H <= 0 || box.X < contentX || box.X+box.W > width || box.Y+box.H > pageContentBottom(6) {
				t.Errorf("width %d: %s out of bounds: %+v", width, name, box)
			}
			if got, want := pageControlPage(name); !want || got != 6 {
				t.Errorf("review control %s does not belong exclusively to review page", name)
			}
			for j := i + 1; j < len(keys); j++ {
				if box.intersects(boxes[keys[j]]) {
					t.Errorf("width %d: %s overlaps %s", width, name, keys[j])
				}
			}
		}
	}
}

func TestReviewBulkCommandsAreWiredAndNoAutoACT(t *testing.T) {
	src, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	win, err := os.ReadFile("candidate_review_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{
		"case ID_REVIEW_SELECT_ALL:", "case ID_REVIEW_SELECT_SEVERE:",
		"case ID_REVIEW_SELECT_WARNING:", "case ID_REVIEW_SELECT_OTHER:",
		"case ID_REVIEW_WHY_DISABLED:", "a.showReviewPublishReason()",
	} {
		if !strings.Contains(string(src), token) {
			t.Errorf("missing native command: %s", token)
		}
	}
	code := string(win)
	start := strings.Index(code, "func (a *state) applyReviewBulkSelection(")
	end := strings.Index(code, "func (a *state) currentReviewPublishGate(")
	if start < 0 || end <= start {
		t.Fatal("bulk handler missing")
	}
	bulk := code[start:end]
	if strings.Contains(bulk, "a.run(") || strings.Contains(bulk, "SubmitForSystem") {
		t.Fatal("selecting checkboxes must not dispatch ACT")
	}
	for _, token := range []string{"a.reviewDetailsOpened = false", "BM_SETCHECK", "a.reviewSelectionSummary()"} {
		if !strings.Contains(bulk, token) {
			t.Errorf("bulk selection missing safety reset: %s", token)
		}
	}
}
