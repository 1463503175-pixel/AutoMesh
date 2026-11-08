package main

import "testing"

func TestReadOnlyRefreshKeepsUserSizesAfterUniqueChineseRename(t *testing.T) {
	old := []Named{{Name: "AM_Hole_Old", IDs: []int{31, 32}}, {Name: "AM_Unique", IDs: []int{33}}}
	now := []Named{{Name: "AM_中文圆柱孔", IDs: []int{32, 31}}, {Name: "AM_Unique", IDs: []int{33}}}
	oldSizes := map[string]float64{"AM_Hole_Old": 1.5, "AM_Unique": 3}
	got, aliases := rebindNamedSettingsByScope(oldSizes, old, now)
	if got["AM_中文圆柱孔"] != 1.5 || got["AM_Unique"] != 3 || aliases["AM_Hole_Old"] != "AM_中文圆柱孔" {
		t.Fatalf("renamed group settings lost: got=%#v aliases=%#v", got, aliases)
	}
}

func TestRefreshCannotRemapAmbiguousOrChangedScope(t *testing.T) {
	old := []Named{{Name: "AM_old", IDs: []int{31}}}
	now := []Named{{Name: "AM_A", IDs: []int{31}}, {Name: "AM_B", IDs: []int{31}}}
	result, _ := rebindNamedSettingsByScope(map[string]float64{"AM_old": 1.5}, old, now)
	if len(result) != 0 {
		t.Fatalf("ambiguous scope renamed silently: %#v", result)
	}
	now = []Named{{Name: "AM_new", IDs: []int{32}}}
	result, _ = rebindNamedSettingsByScope(map[string]float64{"AM_old": 1.5}, old, now)
	if len(result) != 0 {
		t.Fatalf("changed scope renamed silently: %#v", result)
	}
}
