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
		for _, label := range criticalLabels(w) {
			var keys []string
			switch label.Page {
			case 0:
				keys = []string{"auto", "report", "open"}
			case 1:
				keys = []string{"port", "link", "systems", "ack"}
			case 2:
				keys = []string{"size", "global", "finegroups", "finegroupactive", "finegroupmm", "finegroupsave", "finesuggest", "finebulkmm", "finebulkapply", "finenames", "fineprepare", "finegenerate"}
			case 3:
				keys = []string{"skewlow", "skewhigh", "markhighskew", "markwarning", "warningplan", "warningview", "warningpublish", "htmlreport", "diagnose", "mark"}
			case 4:
				keys = []string{"analysisBefore", "analysisAfter", "analysisExport"}
			case 5:
				keys = []string{"export", "exportbutton", "clearlease", "tasklog", "storagechange"}
			}
			for _, key := range keys {
				box := controls[key]
				if key == "systems" {
					box.H = 38
				}
				if label.Area.intersects(box) {
					t.Errorf("%d page%d %s overlaps %s", w, label.Page, label.Name, key)
				}
			}
		}
	}
}

func TestHomeActionsStayInsideStatusCard(t *testing.T) {
	for _, w := range []int{1100, 1240, 1366, 1920} {
		controls := plannedControls(w)
		card := controls["_home_left_card"]
		for _, key := range []string{"report", "open"} {
			b := controls[key]
			if b.X < card.X || b.X+b.W > card.X+card.W || b.Y < card.Y || b.Y+b.H > card.Y+card.H {
				t.Errorf("width %d: %s outside status card", w, key)
			}
		}
	}
}

func TestGlobalActionsStayPinnedAndDoNotOverlap(t *testing.T) {
	for _, w := range []int{940, 1100, 1240, 1600, 1920, 2560} {
		c := plannedControls(w)
		keys := []string{"scan", "qualitycheck", "taskstatus"}
		for _, key := range keys {
			if c[key].Y != 14 || c[key].H != 34 || c[key].X+c[key].W > w {
				t.Errorf("width %d: global action %s has invalid placement %+v", w, key, c[key])
			}
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if c[keys[i]].intersects(c[keys[j]]) {
					t.Errorf("width %d: global actions %s and %s overlap", w, keys[i], keys[j])
				}
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

func TestFineRightColumnDoesNotOverlapListOrClip(t *testing.T) {
	for _, width := range []int{1100, 1120, 1240, 1366, 1600, 1920, 2560} {
		boxes := plannedControls(width)
		list := boxes["finegroups"]
		rightX := boxes["finegroupmm"].X
		if rightX-list.X-list.W < 20 {
			t.Errorf("width %d: no safe gap between list and right panel: list=%+v rightX=%d", width, list, rightX)
		}
		for _, name := range []string{"finegroupactive", "finegroupmm", "finegroupsave"} {
			if list.intersects(boxes[name]) {
				t.Errorf("width %d: list covers %s", width, name)
			}
		}
		for _, label := range criticalLabels(width) {
			if label.Page == 2 && len(label.Name) >= len("右侧") && label.Name[:len("右侧")] == "右侧" {
				if list.intersects(label.Area) {
					t.Errorf("width %d: list hides %s", width, label.Name)
				}
				if label.Area.X+label.Area.W > width-20 {
					t.Errorf("width %d: %s cut off", width, label.Name)
				}
			}
		}
		for _, name := range []string{"finefilter", "finechinese"} {
			b := boxes[name]
			if b.X < list.X || b.X+b.W > list.X+list.W {
				t.Errorf("width %d: %s outside left column", width, name)
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

func TestFineAndReportActionsStayInTheirOwnPages(t *testing.T) {
	for _, width := range []int{1100, 1120, 1280, 1440, 1600, 1920} {
		c := plannedControls(width)
		for _, pair := range [][2]string{
			{"finegroups", "finesuggest"}, {"finebulkmm", "finebulkapply"},
			{"finegroups", "finebulkmm"}, {"finegroups", "finebulkapply"},
			{"finesuggest", "fineprepare"}, {"finebulkapply", "finegenerate"},
			{"mark", "markwarning"}, {"warningplan", "warningpublish"},
		} {
			if c[pair[0]].intersects(c[pair[1]]) {
				t.Errorf("at %d: %s overlaps %s", width, pair[0], pair[1])
			}
		}
	}
}
