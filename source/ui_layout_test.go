package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestAllVisibleControlsFit(t *testing.T) {
	for _, w := range []int{1100, 1120, 1160, 1240, 1280, 1440, 1920, 2560} {
		for name, b := range plannedControls(w) {
			if b.X < 0 || b.Y < 0 || b.W <= 0 || b.H <= 0 || b.X+b.W > w {
				t.Errorf("width %d: %s out of bounds %+v", w, name, b)
			}
			if name != "systems" && name != "htmlreport" && name != "diagnose" && name != "analysisbodylist" && b.Y+b.H > 690 {
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
				keys = []string{"auto", "report", "open"}
			case 1:
				keys = []string{"port", "link", "systems", "ack"}
			case 2:
				keys = []string{"size", "global", "finegroups", "finegroupactive", "finegroupmm", "finegroupsave", "finesuggest", "finebulkmm", "finebulkapply", "finenames", "fineprepare", "finegenerate"}
			case 3:
				keys = []string{"markrule", "markexecute", "skewlow", "skewhigh", "warningplan", "warningview", "warningpublish"}
			case 5:
				keys = []string{"export", "exportbutton", "clearlease", "storagechange", "finechinese"}
			case 4:
				keys = []string{"analysisbase", "analysistarget", "analysisrefresh", "analysisexport", "analysisoverview", "analysiscompare", "analysisdistribution", "analysisregion"}
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
		for _, k := range []string{"report", "open"} {
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
		for _, n := range []string{"finefilter", "finerefresh"} {
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

func TestDeprecatedDuplicateControlsAreSuppressed(t *testing.T) {
	want := map[string]bool{
		"scan": true, "qualitycheck": true, "mark": true, "markwarning": true, "markhighskew": true,
		"taskstatus": true, "taskstatusconn": true, "finebulkmm": true, "finebulkapply": true,
		"finegap": true, "finegapmm": true, "finehole": true, "fineholemm": true,
		"fineport": true, "fineportmm": true, "finecurve": true, "finecurvemm": true, "tasklog": true,
	}
	for _, name := range deprecatedDuplicateControls() {
		if !want[name] {
			t.Errorf("unexpected legacy control in suppression list: %s", name)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("legacy controls not suppressed: %v", want)
	}
}

func TestWorkflowControlsBelongToTheCorrectPage(t *testing.T) {
	cases := map[string]int{
		"auto": 0, "report": 0, "open": 0,
		"port": 1, "link": 1, "initretry": 1, "systems": 1, "ack": 1,
		"size": 2, "global": 2, "finenames": 2, "fineprepare": 2, "finegenerate": 2,
		"finegroups": 2, "finegroupsave": 2, "finerefresh": 2,
		"markrule": 3, "markexecute": 3, "warningplan": 3, "warningview": 3, "warningpublish": 3,
		"htmlreport": 3, "diagnose": 3,
		"analysisbase": 4, "analysistarget": 4, "analysisrefresh": 4, "analysisexport": 4, "analysisreport": 4, "analysisbodylist": 4,
		"export": 5, "exportbutton": 5, "clearlease": 5, "storagechange": 5, "finechinese": 5,
		"reviewlist": 6, "reviewconfirm": 6,
	}
	for name, want := range cases {
		got, ok := pageControlPage(name)
		if !ok || got != want {
			t.Errorf("%s page ownership: got %d, ok=%v, want %d", name, got, ok, want)
		}
	}
	if _, ok := pageControlPage("qualitycheck"); ok {
		t.Fatal("legacy duplicate quality button must not be page-visible")
	}
}

func TestLegacyChineseNamingControlLivesInSettings(t *testing.T) {
	for _, width := range []int{1100, 1120, 1240, 1366, 1600, 1920} {
		controls := plannedControls(width)
		b := controls["finechinese"]
		if b.X < contentX+uiWidth(width)/2 || b.Y < 500 {
			t.Errorf("width %d: legacy naming control is not in Settings advanced-maintenance area: %+v", width, b)
		}
		for _, other := range []string{"storagechange", "clearlease", "tasklog", "export", "exportbutton"} {
			if b.intersects(controls[other]) {
				t.Errorf("width %d: finechinese overlaps %s", width, other)
			}
		}
	}
}

func TestTaskDrawerLogHasUniqueCommandID(t *testing.T) {
	source, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatalf("read Windows UI source: %v", err)
	}
	text := string(source)
	for _, required := range []string{
		"ID_TASK_DRAWER_LOG           = 158",
		"a.put(\"taskdrawerlog\", button(\"打开任务日志\", 0, 0, 140, 32, ID_TASK_DRAWER_LOG))",
		"case ID_COPY, ID_TASK_DRAWER_LOG:",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("missing unique task drawer log routing: %s", required)
		}
	}
}

func TestDirectButtonCommandIDsAreUnique(t *testing.T) {
	source, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatalf("read Windows UI source: %v", err)
	}
	re := regexp.MustCompile(`a\.put\("([^"]+)"\s*,\s*button\("[^"]*"[^\n]*?\b(ID_[A-Z0-9_]+)\)`)
	matches := re.FindAllStringSubmatch(string(source), -1)
	owners := map[string]string{}
	for _, match := range matches {
		name, id := match[1], match[2]
		if previous, exists := owners[id]; exists {
			t.Errorf("button command ID %s is shared by %s and %s", id, previous, name)
		} else {
			owners[id] = name
		}
	}
	if owners["ID_TASK_DRAWER_LOG"] != "taskdrawerlog" {
		t.Errorf("task drawer log command ID owner = %q", owners["ID_TASK_DRAWER_LOG"])
	}
	if owners["ID_QUALITY_CHECK"] == "qualitycheck" {
		t.Fatal("legacy quality button must not share the active global quality action ID")
	}
	if owners["ID_TASK_STATUS"] == "taskstatus" || owners["ID_TASK_STATUS"] == "taskstatusconn" {
		t.Fatal("legacy task status controls must not share an active action ID")
	}
}

func TestFinerefreshUsesScanAvailabilityNotExistingSnapshot(t *testing.T) {
	source, err := os.ReadFile("ui_action_state_windows.go")
	if err != nil {
		t.Fatalf("read action-state source: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, `enabled(a.ctl("finerefresh"), available.Allow("scan"))`) {
		t.Fatal("full geometry scan must use the pre-scan 'scan' availability gate")
	}
	if strings.Contains(text, `"finefilter", "finerefresh"`) {
		t.Fatal("full geometry scan must not be disabled by the existing-snapshot editing gate")
	}
}
