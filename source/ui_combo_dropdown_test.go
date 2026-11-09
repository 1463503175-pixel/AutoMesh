package main

import "testing"

func TestNativeCombosReserveDropdownHeight(t *testing.T) {
	for _, width := range []int{1100, 1280, 1600} {
		controls := plannedControls(width)
		for _, name := range []string{"markrule", "analysisbase", "analysistarget", "systems"} {
			b := controls[name]
			if got := nativeComboWindowHeight(name, b.H); got < 190 {
				t.Errorf("width %d combo %s: HWND height %d clips dropdown", width, name, got)
			}
			if b.H > 38 && name != "systems" {
				t.Errorf("width %d combo %s: collapsed layout unexpectedly occupies %d", width, name, b.H)
			}
		}
	}
	if got := nativeComboWindowHeight("markexecute", 34); got != 34 {
		t.Fatalf("non-combobox button height changed: %d", got)
	}
}
