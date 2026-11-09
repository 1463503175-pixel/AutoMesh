//go:build windows

package main

import "time"

// sharedDataRuntime stores only freshness/identity metadata. Engineering,
// geometry and quality payloads continue to live in the existing state fields;
// this prevents page-specific copies of the same data.
type sharedDataRuntime struct {
	engineeringSystem  string
	engineeringStale   bool
	engineeringError   string
	geometryScannedAt  time.Time
	geometryStale      bool
	geometryError      string
	qualitySystem      string
	qualityError       string
	connectionError    string
	lastSelectedSystem string
}

func (a *state) selectedMeshSystem() string {
	if a.ctl("systems") == 0 {
		return ""
	}
	index := int(send(a.ctl("systems"), CB_GETCURSEL, 0, 0))
	if index < 0 || index >= len(a.systems) {
		return ""
	}
	return a.systems[index]
}

func (a *state) sharedDataStatus() SharedDataView {
	return deriveSharedDataView(SharedDataInput{
		Connected: a.connected, Connecting: a.initRunning,
		ConnectionError: a.shared.connectionError, InitError: a.initError,
		Busy: a.busy, ActivePhase: a.activePhase, CurrentSystem: a.selectedMeshSystem(),
		EngineeringSummary: a.engineeringSummary, EngineeringSystem: a.shared.engineeringSystem,
		EngineeringAt: a.engineeringSyncedAt, EngineeringStale: a.shared.engineeringStale,
		EngineeringError: a.shared.engineeringError,
		Geometry:         a.snapshot, GeometrySystem: a.scannedSystem,
		GeometryScannedAt: a.shared.geometryScannedAt, GeometryStale: a.shared.geometryStale,
		GeometryError:  a.shared.geometryError,
		QualitySummary: a.qualitySummary, QualityPresent: a.qualityResult != nil,
		QualitySystem: a.shared.qualitySystem, QualityAt: a.qualityCheckedAt,
		QualityStale: a.qualityStale, QualityError: a.shared.qualityError,
	})
}

// onMeshSystemSelectionChanged preserves old data for traceability but marks
// every payload from another Mesh system stale. Switching back does not silently
// reactivate it; a fresh sync/scan/check is required to restore validity.
func (a *state) onMeshSystemSelectionChanged() {
	selected := a.selectedMeshSystem()
	if selected == "" {
		if a.shared.lastSelectedSystem != "" {
			if len(a.engineeringSummary) > 0 {
				a.shared.engineeringStale = true
			}
			if a.snapshot != nil || a.scannedSystem != "" {
				a.shared.geometryStale = true
			}
			if a.qualityResult != nil {
				a.qualityStale = true
			}
		}
		a.shared.lastSelectedSystem = ""
		a.updateActionStates()
		refresh()
		return
	}
	if a.shared.lastSelectedSystem != "" && selected != a.shared.lastSelectedSystem {
		if len(a.engineeringSummary) > 0 && a.shared.engineeringSystem != selected {
			a.shared.engineeringStale = true
		}
		if (a.snapshot != nil || a.scannedSystem != "") && a.scannedSystem != selected {
			a.shared.geometryStale = true
		}
		if a.qualityResult != nil && a.shared.qualitySystem != selected {
			a.qualityStale = true
		}
	}
	a.shared.lastSelectedSystem = selected
	a.updateActionStates()
	refresh()
}

func sharedStateLabel(state SharedDataState) string {
	switch state {
	case SharedStateLoading:
		return "读取中"
	case SharedStateReady:
		return "有效"
	case SharedStateStale:
		return "待更新"
	case SharedStateFailed:
		return "失败"
	case SharedStatePartial:
		return "部分"
	default:
		return "未读取"
	}
}
