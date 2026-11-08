package main

import "testing"

func TestGroupToggleRetainsIndividualSizesAcrossSwitches(t *testing.T) {
	active := map[string]float64{"A": 1.4, "B": 2.2, "C": 8}
	paused := map[string]float64{}
	group := []string{"A", "B"}
	setBucketEnabled(active, paused, group, nil, 1.5, false)
	if len(active) != 1 || active["C"] != 8 || paused["A"] != 1.4 || paused["B"] != 2.2 {
		t.Fatalf("disabled group lost per-item sizes: active=%v, paused=%v", active, paused)
	}
	// Simulates switching the selected list row; state must live in maps,
	// never in the checkbox's transient BM_GETCHECK value.
	setBucketEnabled(active, paused, group, nil, 1.5, true)
	if active["A"] != 1.4 || active["B"] != 2.2 || active["C"] != 8 {
		t.Fatalf("reenabled group did not restore manual per-item mm: %v", active)
	}
}
