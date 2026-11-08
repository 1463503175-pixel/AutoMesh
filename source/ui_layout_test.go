package main

import "testing"

func TestAllVisibleControlsFit(t *testing.T) {
	for _, w := range []int{1100, 1120, 1160, 1240, 1280, 1440, 1920, 2560} {
		for name, b := range plannedControls(w) {
			if b.X < 0 || b.Y < 0 || b.W <= 0 || b.H <= 0 || b.X+b.W > w {
				t.Errorf("width %d: %s out of bounds %+v", w, name, b)
			}
			if name != "systems" && b.Y+b.H > 690 {
				t.Errorf("height 690: %s out of bounds %+v", name, b)
			}
		}
	}
}
func TestCriticalTextDoesNotOverlapControls(t *testing.T) {
	for _, w := range []int{1100, 1120, 1240, 1366, 1600, 1920} {
		controls := plannedControls(w)
		for _, l := range criticalLabels(w) {
			var keys []string
			switch l.Page {
			case 0:
				keys = []string{"auto", "size", "global", "scan", "report", "open"}
			case 1:
				keys = []string{"port", "link", "systems", "ack"}
			case 2:
				keys = []string{"finegroups", "finegroupactive", "finegroupmm", "finegroupsave", "finesuggest", "finebulkmm", "finebulkapply", "finenames", "fineprepare", "finegenerate"}
			case 3:
				keys = []string{"qualitycheck", "htmlreport", "diagnose", "mark", "markwarning", "warningplan", "warningview", "warningpublish"}
			case 4:
				keys = []string{"export", "exportbutton", "clearlease"}
			}
			for _, k := range keys {
				box := controls[k]
				if k == "systems" {
					box.H = 38
				}
				// The Named Selection listbox is always visible and occupies its true height.
				if l.Area.intersects(box) {
					t.Errorf("%d page%d %s overlaps %s", w, l.Page, l.Name, k)
				}
			}
		}
	}
}
func TestButtonsInsideHomeCard(t *testing.T) {
	for _, w := range []int{1100, 1240, 1366, 1920} {
		c := plannedControls(w)
		card := c["_home_left_card"]
		for _, k := range []string{"scan", "report", "open"} {
			b := c[k]
			if b.X < card.X || b.X+b.W > card.X+card.W || b.Y < card.Y || b.Y+b.H > card.Y+card.H {
				t.Errorf("width %d: %s outside status card", w, k)
			}
		}
	}
}
func TestNoSecurityControlOrAdvancedOptions(t *testing.T) {
	if _, ok := plannedControls(1240)["security"]; ok {
		t.Fatal("security selector should not be visible")
	}
	if _, ok := plannedControls(1240)["runcommand"]; ok {
		t.Fatal("advanced command button should not be visible")
	}
}

func TestThreePublicationActionsDoNotOverlap(t *testing.T) {
	for _, width := range []int{1100, 1120, 1240, 1600, 1920} {
		controls := plannedControls(width)
		keys := []string{"finenames", "fineprepare", "finegenerate"}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if controls[keys[i]].intersects(controls[keys[j]]) {
					t.Errorf("at %d: %s overlaps %s", width, keys[i], keys[j])
				}
			}
		}
	}
}

// Direct-to-existing-NS workflow: list, editing fields and action buttons
// must never overlap, including at the minimum supported window width.
func TestNamedSelectionListAndControlsDoNotOverlap(t *testing.T) {
	for _, w := range []int{1100, 1120, 1240, 1600, 1920} {
		c := plannedControls(w)
		keys := []string{"finegroups", "finegroupactive", "finegroupmm", "finegroupsave", "finenames", "fineprepare", "finegenerate"}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if c[keys[i]].intersects(c[keys[j]]) {
					t.Errorf("width %d: %s overlaps %s", w, keys[i], keys[j])
				}
			}
		}
	}
}

// The listbox must never cover the right-hand editor, including its labels
// and guidance, not merely the child window controls.
func TestFineRightColumnDoesNotOverlapListOrClip(t *testing.T) {
	for _, width := range []int{1100, 1120, 1240, 1366, 1600, 1920, 2560} {
		boxes := plannedControls(width)
		list := boxes["finegroups"]
		rightX := boxes["finegroupmm"].X
		if rightX-list.X-list.W < 20 {
			t.Errorf("width %d: no safe gap between list and right panel: list=%+v rightX=%d", width, list, rightX)
		}
		for _, n := range []string{"finegroupactive", "finegroupmm", "finegroupsave"} {
			if list.intersects(boxes[n]) {
				t.Errorf("width %d: list covers %s", width, n)
			}
		}
		for _, lab := range criticalLabels(width) {
			if lab.Page == 2 && len(lab.Name) >= len("右侧") && lab.Name[:len("右侧")] == "右侧" {
				if list.intersects(lab.Area) {
					t.Errorf("width %d: list hides %s", width, lab.Name)
				}
				if lab.Area.X+lab.Area.W > width-20 {
					t.Errorf("width %d: %s cut off", width, lab.Name)
				}
			}
		}
		for _, n := range []string{"finefilter", "finerefresh", "finechinese"} {
			b := boxes[n]
			if b.X < list.X || b.X+b.W > list.X+list.W {
				t.Errorf("width %d: %s outside left column", width, n)
			}
		}
	}
}

func TestRefreshPreservesOnlyValidNamedPreferences(t *testing.T) {
	old := map[string]float64{"4inch": 3.5, "AM_旧组": 1.5}
	groups := []Named{{Name: "4inch", IDs: []int{1, 2}}, {Name: "AM_旧组", IDs: nil}}
	sizes := preservedNamedSizes(old, "project-A|hash-1", "project-A|hash-1", groups)
	if len(sizes) != 1 || sizes["4inch"] != 3.5 {
		t.Fatalf("incorrect refresh reconciliation: %+v", sizes)
	}
	changed := preservedNamedSizes(old, "project-A|hash-1", "project-A|hash-2", groups)
	if len(changed) != 0 {
		t.Fatalf("must clear preferences when geometry changes: %+v", changed)
	}
}

func TestV105NewControlsDoNotOverlap(t *testing.T) {
	for _, width := range []int{1100, 1120, 1280, 1440, 1600, 1920, 2560} {
		controls := plannedControls(width)
		for _, pair := range [][2]string{{"finegroups", "finesuggest"}, {"finebulkmm", "finebulkapply"}, {"finegroups", "finebulkmm"}, {"finegroups", "finebulkapply"}, {"finesuggest", "fineprepare"}, {"finebulkapply", "finegenerate"}, {"mark", "markwarning"}, {"qualitycheck", "markwarning"}} {
			a, b := controls[pair[0]], controls[pair[1]]
			if a.intersects(b) {
				t.Errorf("at %d: %s overlaps %s: %+v %+v", width, pair[0], pair[1], a, b)
			}
		}
		for _, name := range []string{"finesuggest", "finebulkmm", "finebulkapply", "markwarning"} {
			b := controls[name]
			if b.X < 0 || b.X+b.W > width || b.Y+b.H > 690 {
				t.Errorf("%d: %s outside %v", width, name, b)
			}
		}
	}
}

func TestWarningHotspotActionsDoNotOverlap(t *testing.T) {
	for _, w := range []int{1100, 1240, 1366, 1920} {
		c := plannedControls(w)
		for _, old := range []string{"qualitycheck", "htmlreport", "diagnose", "mark", "markwarning"} {
			for _, extra := range []string{"warningplan", "warningview", "warningpublish"} {
				if c[old].intersects(c[extra]) {
					t.Errorf("%d: %s intersects %s", w, old, extra)
				}
			}
		}
		if c["warningplan"].intersects(c["warningpublish"]) {
			t.Errorf("%d: new actions overlap", w)
		}
	}
}
