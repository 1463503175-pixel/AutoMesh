package main

import (
	"math"
	"testing"
)

func testGroupSnap() *Snapshot {
	return &Snapshot{
		GeometryUnit: "mm", Mesh: map[string]any{"ElementSize": "8 [mm]"},
		Faces: []Face{
			{ID: 1, Area: 9, Type: "Cylinder"}, {ID: 2, Area: 9.2, Type: "Cylinder"},
			{ID: 3, Area: 9, Type: "Cylinder"}, {ID: 4, Area: 9, Type: "Cylinder"},
			{ID: 5, Area: 1200, Type: "Plane"}, {ID: 6, Area: 1300, Type: "Plane"},
		}, Named: []Named{
			{Name: "AM_小孔组A", IDs: []int{1, 2}},
			{Name: "AM_小孔组B", IDs: []int{3, 4}},
			{Name: "4inch", IDs: []int{5, 6}},
			{Name: "inlet", IDs: []int{1}},
		},
	}
}
func TestRecommendationsAreRealNamedAndKeepBoundaries(t *testing.T) {
	s := testGroupSnap()
	a := recommendNamedSizes(s, 8)
	if len(a) != 3 {
		t.Fatalf("wanted 3 groups, got %#v", a)
	}
	rec := map[string]float64{}
	for _, x := range a {
		rec[x.Name] = x.MM
		if x.MM >= 8 {
			t.Errorf("named group needs preliminary refinement %s: %g", x.Name, x.MM)
		}
	}
	if rec["4inch"] < 3 || rec["4inch"] > 4 {
		t.Errorf("repeat wafer-sized faces: %.3f", rec["4inch"])
	}
	if _, ok := rec["inlet"]; ok {
		t.Fatal("inlet should never be recommended")
	}
	b := groupAdvice(a, nil)
	if len(b) < 2 {
		t.Fatalf("need coarse/fine size groups, got %#v", b)
	}
}
func TestManualPerItemOverrideAndGroupSeparation(t *testing.T) {
	a := recommendNamedSizes(testGroupSnap(), 8)
	b := groupAdvice(a, nil)
	sizes := map[string]float64{"AM_小孔组A": 0.8}
	override := groupAdvice(a, sizes)
	if math.Abs(override[0].MM-0.8) > 0.01 {
		t.Fatalf("override not represented: %+v", override)
	}
	if len(b[0].Members) == 0 {
		t.Fatal("lost members")
	}
}
func TestDefaultAutoActivationNeverOverlaps(t *testing.T) {
	s := testGroupSnap()
	s.Named = append(s.Named, Named{Name: "AM_重复同面", IDs: []int{1, 2}})
	a := recommendNamedSizes(s, 8)
	enabled := advisedActivation(a, s)
	used := map[int]bool{}
	for name := range enabled {
		for _, n := range s.Named {
			if n.Name == name {
				for _, id := range n.IDs {
					if used[id] {
						t.Fatalf("overlap ID%d", id)
					}
					used[id] = true
				}
			}
		}
	}
}
func TestGlobalUnitConversionAndUnknown(t *testing.T) {
	s := &Snapshot{Mesh: map[string]any{"ElementSize": "0.008 [m]"}}
	if displayedGlobalMM(s, 12) != 8 {
		t.Fatal("meters conversion")
	}
	s.Mesh["ElementSize"] = "bad"
	if displayedGlobalMM(s, 12) != 12 {
		t.Fatal("fallback only if unreadable")
	}
}
func TestSurfacePreviewIsNotMistakenForCellCount(t *testing.T) {
	s := testGroupSnap()
	v, overlap, who := previewSurfaceBudget(s, map[string]float64{"AM_小孔组A": 1})
	if v < 18 || v > 19 {
		t.Fatalf("bad area proxy %.3f", v)
	}
	if overlap != 0 || who != "AM_小孔组A" {
		t.Fatalf("bad scope %d %s", overlap, who)
	}
	s.Named = append(s.Named, Named{Name: "AM_重复同面", IDs: []int{1, 2}})
	v2, overlap, _ := previewSurfaceBudget(s, map[string]float64{"AM_小孔组A": 1, "AM_重复同面": 0.5})
	if overlap != 2 {
		t.Fatalf("must detect overlapping faces; got %d", overlap)
	}
	if v2 <= v {
		t.Fatalf("finer overlapping sizing must increase conservative cost %.2f <= %.2f", v2, v)
	}
}

func TestBulkSizeNeverChangesSelectionAndOnRestoresSavedSize(t *testing.T) {
	active := map[string]float64{"A": 1.5}
	paused := map[string]float64{"B": 3.2, "C": 8}
	setBucketSizePreservingSelection(active, paused, []string{"A", "B", "C"}, 5.2)
	if len(active) != 1 || active["A"] != 5.2 || paused["B"] != 5.2 || paused["C"] != 5.2 {
		t.Fatalf("modified state: %v %v", active, paused)
	}
	setBucketEnabled(active, paused, []string{"A", "B", "C"}, nil, 8, true)
	if len(active) != 3 || active["B"] != 5.2 || active["C"] != 5.2 {
		t.Fatalf("restored state: %v", active)
	}
	setBucketEnabled(active, paused, []string{"A", "B", "C"}, nil, 8, false)
	if len(active) != 0 || paused["A"] != 5.2 {
		t.Fatalf("disabled state: %v %v", active, paused)
	}
}
