package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Check the saved marker's project/geometry identity and recorded element
// count before offering a new preview. This is a UI affordance check only:
// the ACT Worker must STILL verify actual mesh Element/Named Selection IDs.
func markerForCurrentSnapshot(m SkewMarker, current *Snapshot) bool {
	if current == nil || m.Status != "valid" || current.Project == "" || current.Fingerprint == "" ||
		m.Project != current.Project || m.Fingerprint != current.Fingerprint ||
		m.WorksheetCount <= 0 || m.MeshCount <= 0 || len(m.Groups) == 0 {
		return false
	}
	if current.Mesh == nil {
		return false
	}
	cells, err := meshCellCount(current.Mesh["Elements"])
	if err != nil || cells != m.MeshCount {
		return false
	}
	counted := 0
	unique := make(map[int]struct{}, m.WorksheetCount)
	for _, group := range m.Groups {
		if strings.TrimSpace(group.Name) == "" || group.ObjectID <= 0 || len(group.IDs) == 0 {
			return false
		}
		counted += len(group.IDs)
		for _, id := range group.IDs {
			if id <= 0 {
				return false
			}
			if _, seen := unique[id]; seen {
				return false
			}
			unique[id] = struct{}{}
		}
	}
	return counted == m.WorksheetCount
}

func (w Workspace) hasCurrentMarker(s *Snapshot) bool {
	// readJSON handles malformed/truncated input without fabricating success.
	m, err := readJSON[SkewMarker](w.path("warning_refinement_marker.json"))
	return err == nil && markerForCurrentSnapshot(m, s)
}

func validSkewRange(lowText, highText string) bool {
	a, e1 := strconv.ParseFloat(strings.TrimSpace(lowText), 64)
	b, e2 := strconv.ParseFloat(strings.TrimSpace(highText), 64)
	return e1 == nil && e2 == nil && !math.IsNaN(a) && !math.IsNaN(b) && !math.IsInf(a, 0) && !math.IsInf(b, 0) && a >= 0 && b <= 1 && a < b
}

// An HTML file existing on disk does not prove it describes the active Mesh.
func (w Workspace) hasCurrentWarningReport(s *Snapshot) bool {
	if s == nil {
		return false
	}
	if _, err := os.Stat(w.path("warning_refinement_plan.html")); err != nil {
		return false
	}
	_, err := w.ReadReviewedWarningDiagnostic(s)
	return err == nil
}

// JSON decodes integral ACT mesh counts to float64 by default. fmt.Sprint
// formats 10,055,480 as 1.005548e+07; strconv.Atoi cannot parse that. Never
// silently round a fractional or invalid count when validating live identity.
func meshCellCount(raw any) (int, error) {
	switch v := raw.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > float64(int(^uint(0)>>1)) || math.Trunc(v) != v {
			return 0, fmt.Errorf("invalid mesh count: %v", v)
		}
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil || i < 0 || uint64(i) > uint64(^uint(0)>>1) {
			return 0, fmt.Errorf("invalid mesh count: %v", v)
		}
		return int(i), nil
	case int:
		if v < 0 {
			return 0, fmt.Errorf("negative mesh count")
		}
		return v, nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || i < 0 {
			return 0, fmt.Errorf("invalid mesh count: %q", v)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("unsupported mesh count type: %T", raw)
	}
}
