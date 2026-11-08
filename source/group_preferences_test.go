package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func prefsSnapshot() *Snapshot {
	return &Snapshot{Project: `E:\ALD\10.22`, Fingerprint: "geometry-AAA"}
}
func prefsNamed() []Named {
	return []Named{
		{Name: "Fine_A", IDs: []int{3, 1}},
		{Name: "Fine_B", IDs: []int{4}},
		{Name: "Coarse_8mm_A", IDs: []int{10, 9}},
		{Name: "Coarse_8mm_B", IDs: []int{11}},
		{Name: "inlet", IDs: []int{12}},
	}
}
func TestEightMMDisabledSurvivesRestartAndRescan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "group_preferences_v1.json")
	s := prefsSnapshot()
	names := prefsNamed()
	active := map[string]float64{"Fine_A": 1.25, "Fine_B": 3, "Coarse_8mm_A": 8, "Coarse_8mm_B": 8}
	paused := map[string]float64{}
	setBucketEnabled(active, paused, []string{"Coarse_8mm_A", "Coarse_8mm_B"}, nil, 8, false)
	if bucketEnabledCount(active, []string{"Coarse_8mm_A", "Coarse_8mm_B"}) != 0 {
		t.Fatal("8mm group should be disabled")
	}
	p, err := makeSavedNamedProfile(s, names, active, paused)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveNamedPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	// Simulate a brand-new process, and ACT returning the same face scopes in a new ordering.
	newNames := prefsNamed()
	newNames[2].IDs = []int{9, 10}
	got, off, count, found, err := loadNamedPreferences(path, s, newNames)
	if err != nil || !found || count != 4 {
		t.Fatalf("restore: %v found=%v matched=%d", err, found, count)
	}
	if len(got) != 2 || got["Fine_A"] != 1.25 || got["Fine_B"] != 3 || off["Coarse_8mm_A"] != 8 || off["Coarse_8mm_B"] != 8 {
		t.Fatalf("unexpected restored selections: active=%v paused=%v", got, off)
	}
	setBucketEnabled(got, off, []string{"Coarse_8mm_A", "Coarse_8mm_B"}, nil, 8, true)
	if got["Coarse_8mm_A"] != 8 || got["Coarse_8mm_B"] != 8 {
		t.Fatalf("manual mm lost after on: %v", got)
	}
}
func TestPersistRenamedExactUniqueScopeOnly(t *testing.T) {
	s := prefsSnapshot()
	p, err := makeSavedNamedProfile(s, prefsNamed(), map[string]float64{"Fine_A": 1.6}, map[string]float64{"Coarse_8mm_A": 8})
	if err != nil {
		t.Fatal(err)
	}
	active, paused, count, err := restoreSavedNamedProfile(p, s, []Named{{Name: "Renamed_Fine_A", IDs: []int{1, 3}}, {Name: "Renamed_Coarse", IDs: []int{10, 9}}})
	if err != nil || count != 2 || active["Renamed_Fine_A"] != 1.6 || paused["Renamed_Coarse"] != 8 {
		t.Fatalf("rename exact scope: %v %v %v %d", err, active, paused, count)
	}
	active, paused, count, err = restoreSavedNamedProfile(p, s, []Named{{Name: "Renamed_Fine_A", IDs: []int{1, 3}}, {Name: "Same_scope", IDs: []int{1, 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || len(active) != 0 || len(paused) != 0 {
		t.Fatalf("ambiguous scope restored unexpectedly: %v %d", active, count)
	}
}
func TestNoReuseChangedScopeOrDifferentGeometry(t *testing.T) {
	s := prefsSnapshot()
	p, err := makeSavedNamedProfile(s, prefsNamed(), map[string]float64{"Fine_A": 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	active, _, matched, err := restoreSavedNamedProfile(p, s, []Named{{Name: "Fine_A", IDs: []int{1, 7}}})
	if err != nil || matched != 0 || len(active) != 0 {
		t.Fatal("changed scope retained", active, matched, err)
	}
	changed := *s
	changed.Fingerprint = "geometry-BBB"
	_, _, _, err = restoreSavedNamedProfile(p, &changed, prefsNamed())
	if err == nil {
		t.Fatal("cross geometry restore accepted")
	}
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := saveNamedPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	_, _, _, found, err := loadNamedPreferences(path, &changed, prefsNamed())
	if err != nil || found {
		t.Fatal("different geometry loaded", found, err)
	}
}
func TestMultipleProjectsPersistAndKeepBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pref.json")
	s := prefsSnapshot()
	p, _ := makeSavedNamedProfile(s, prefsNamed(), map[string]float64{"Fine_A": 1.1}, nil)
	if err := saveNamedPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	other := *s
	other.Project = `D:\Other`
	q, _ := makeSavedNamedProfile(&other, prefsNamed(), map[string]float64{"Fine_B": 2.8}, nil)
	if err := saveNamedPreferences(path, q); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("no recovery backup", err)
	}
	v, err := readNamedProfiles(path)
	if err != nil || len(v.Profiles) != 2 {
		t.Fatalf("profile count %d err=%v", len(v.Profiles), err)
	}
}
func TestCorruptFileNeverOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	p, _ := makeSavedNamedProfile(prefsSnapshot(), prefsNamed(), map[string]float64{"Fine_A": 1}, nil)
	if err := saveNamedPreferences(path, p); err == nil {
		t.Fatal("should refuse overwrite of corrupt file")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "{broken" {
		t.Fatal("corrupt file silently overwritten")
	}
}
func TestRejectNanAndInf(t *testing.T) {
	s := prefsSnapshot()
	p, err := makeSavedNamedProfile(s, prefsNamed(), map[string]float64{"Fine_A": math.NaN(), "Fine_B": math.Inf(1)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Selections["Fine_A"].SizeMM != 0 || p.Selections["Fine_B"].SizeMM != 0 {
		t.Fatal("nonfinite size persisted")
	}
}
