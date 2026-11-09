package main

import (
	"strings"
	"testing"
)

func TestNamedBoundaryRecommendationAndCoverage(t *testing.T) {
	s := &Snapshot{GeometryUnit: "mm", Mesh: map[string]any{"ElementSize": "12 [mm]"},
		Faces:            []Face{{ID: 11, Area: 706.8, Type: "Plane"}, {ID: 12, Area: 1200, Type: "Plane"}},
		Named:            []Named{{Name: "inlet", IDs: []int{11}}, {Name: "outlet", IDs: []int{12}}},
		ControlsComplete: true, Controls: []map[string]any{{"name": "UserInletSizing", "category": "Sizing", "is_sizing": true, "scope_ids": []any{float64(11)}, "size_mm": 3.2}}}
	advice := recommendNamedSizes(s, 12)
	if len(advice) != 2 {
		t.Fatalf("boundaries lost: %+v", advice)
	}
	for _, a := range advice {
		if a.Eligible || a.MM <= 0 || a.MM >= 12 {
			t.Fatalf("bad boundary default: %+v", a)
		}
	}
	if got := namedSizingCoverage(s, s.Named[0]); !strings.Contains(got, "3.2 mm") {
		t.Fatalf("existing Sizing not shown: %s", got)
	}
	if got := namedSizingCoverage(s, s.Named[1]); got != "未覆盖" {
		t.Fatal(got)
	}
	s.Controls[0]["scope_ids"] = []any{float64(11), float64(12)}
	if got := namedSizingCoverage(s, s.Named[0]); !strings.Contains(got, "交叠") {
		t.Fatal(got)
	}
	s.ControlsComplete = false
	if got := namedSizingCoverage(s, s.Named[0]); got != "覆盖待核验" {
		t.Fatal(got)
	}
}

func TestBoundaryExistingSizeReadbackDoesNotInferFromArea(t *testing.T) {
	s := &Snapshot{ControlsComplete: true, Controls: []map[string]any{
		{"name": "ExistingUserInlet", "category": "Sizing", "is_sizing": true, "scope_ids": []any{float64(11)}, "size_mm": 1.7},
	}}
	n := Named{Name: "inlet", IDs: []int{11}}
	mm, ok := existingNamedSizingSize(s, n)
	if !ok || mm != 1.7 {
		t.Fatalf("existing native mm must win: %.4f %v", mm, ok)
	}
	s.Controls = append(s.Controls, map[string]any{"name": "duplicate", "category": "Sizing", "scope_ids": []any{float64(11)}, "size_mm": 2.2})
	if _, ok = existingNamedSizingSize(s, n); ok {
		t.Fatal("ambiguous scope must not be used as default")
	}
}

func TestInletOutletSeparatedFromAutomaticSizeBuckets(t *testing.T) {
	snap := &Snapshot{GeometryUnit: "mm", Mesh: map[string]any{"ElementSize": "12 [mm]"},
		Faces: []Face{{ID: 11, Area: 100, Type: "Plane"}, {ID: 12, Area: 400, Type: "Plane"}, {ID: 31, Area: 60, Type: "Cylinder"}},
		Named: []Named{{Name: "inlet", IDs: []int{11}}, {Name: "outlet", IDs: []int{12}}, {Name: "AM_Hole", IDs: []int{31}}}}
	advice := recommendNamedSizes(snap, 12)
	groups := groupAdvice(advice, nil)
	if len(groups) < 3 {
		t.Fatalf("missing separate boundary groups: %+v", groups)
	}
	if groups[0].Key != "boundary:inlet" || groups[1].Key != "boundary:outlet" {
		t.Fatalf("boundary groups not first: %+v", groups)
	}
	if len(groups[0].Members) != 1 || len(groups[1].Members) != 1 {
		t.Fatal("boundary bulk edit would accidentally change unrelated names")
	}
	if groups[0].MM == groups[1].MM {
		t.Fatal("different areas must produce independent suggestions")
	}
}
