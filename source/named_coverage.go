package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// namedSizingCoverage is an advisory projection of the last READ-ONLY ACT
// inventory. Live ACT validation remains authoritative before publication.
// Do not infer a Sizing from a matching name alone: require verified face ids.
func namedSizingCoverage(s *Snapshot, named Named) string {
	if s == nil || !s.ControlsComplete {
		return "覆盖待核验"
	}
	wanted := map[int]bool{}
	for _, id := range named.IDs {
		wanted[id] = true
	}
	if len(wanted) == 0 {
		return "作用面失效"
	}
	status := "未覆盖"
	for _, c := range s.Controls {
		if !controlIsSizing(c) || boolAny(c["suppressed"]) {
			continue
		}
		raw, ok := c["scope_ids"]
		if !ok {
			status = "覆盖不明（检查已有控制）"
			continue
		}
		scope := scopeInts(raw)
		if len(scope) == 0 {
			status = "覆盖不明（检查已有控制）"
			continue
		}
		overlap := 0
		for _, id := range scope {
			if wanted[id] {
				overlap++
			}
		}
		if overlap == 0 {
			continue
		}
		label := strings.TrimSpace(fmt.Sprint(c["name"]))
		if overlap != len(wanted) || len(scope) != len(wanted) {
			status = "部分交叠（检查 " + coverageLabel(label, 22) + "）"
			continue
		}
		if mm, ok := numberAny(c["size_mm"]); ok && mm > 0 && !math.IsInf(mm, 0) {
			return fmt.Sprintf("已有 Sizing %s · %.3g mm", coverageLabel(label, 18), mm)
		}
		return "已有 Sizing " + coverageLabel(label, 24)
	}
	return status
}

func controlIsSizing(c map[string]any) bool {
	if boolAny(c["is_sizing"]) {
		return true
	}
	return strings.Contains(strings.ToLower(fmt.Sprint(c["category"])), "sizing")
}
func boolAny(v any) bool {
	b, ok := v.(bool)
	return ok && b
}
func numberAny(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}
func scopeInts(v any) []int {
	out := []int{}
	switch a := v.(type) {
	case []int:
		out = append(out, a...)
	case []any:
		for _, x := range a {
			if f, ok := numberAny(x); ok && f == math.Trunc(f) {
				out = append(out, int(f))
			}
		}
	}
	sort.Ints(out)
	return out
}

func coverageLabel(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// A user-boundary suggestion must not silently replace an already-applied
// Sizing's native value. Only trust an exact, uniquely scoped live control.
func existingNamedSizingSize(s *Snapshot, named Named) (float64, bool) {
	if s == nil || !s.ControlsComplete || len(named.IDs) == 0 {
		return 0, false
	}
	expected := map[int]bool{}
	for _, id := range named.IDs {
		expected[id] = true
	}
	var mm float64
	found := 0
	for _, c := range s.Controls {
		if !controlIsSizing(c) || boolAny(c["suppressed"]) {
			continue
		}
		scope := scopeInts(c["scope_ids"])
		if len(scope) != len(expected) {
			continue
		}
		same := true
		for _, id := range scope {
			if !expected[id] {
				same = false
				break
			}
		}
		if !same {
			continue
		}
		v, ok := numberAny(c["size_mm"])
		if !ok || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		found++
		mm = v
	}
	return mm, found == 1
}
