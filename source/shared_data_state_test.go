package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSharedDataViewSeparatesEngineeringGeometryAndQuality(t *testing.T) {
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	geometry := &Snapshot{Project: "E:/sim/model", Fingerprint: "geo-A"}
	input := SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		EngineeringSummary: map[string]any{"project": "E:/sim/model", "body_count": "8"},
		EngineeringSystem:  "FFF", EngineeringAt: at,
		Geometry: geometry, GeometrySystem: "FFF", GeometryScannedAt: at,
		QualitySummary: map[string]any{"project": "E:/sim/model", "geometry_fingerprint": "geo-A", "quality": map[string]any{"skewness": map[string]any{"max": 0.91}, "orthogonal_quality": map[string]any{"min": 0.12}}},
		QualityPresent: true, QualitySystem: "FFF", QualityAt: at,
	}
	view := deriveSharedDataView(input)
	if view.ConnectionState != SharedStateReady || view.EngineeringState != SharedStateReady || view.GeometryState != SharedStateReady || view.QualityState != SharedStateReady {
		t.Fatalf("expected all domains ready: %+v", view)
	}
	if view.Project != "E:/sim/model" || view.MeshSystem != "FFF" {
		t.Fatalf("incorrect shared identity: %+v", view)
	}
}

func TestSharedDataViewDoesNotTreatSyncAsGeometryScan(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		EngineeringSummary: map[string]any{"project": "E:/sim/model", "body_count": "8"},
		EngineeringSystem:  "FFF", EngineeringAt: time.Now(),
	})
	if view.EngineeringState != SharedStateReady {
		t.Fatalf("engineering sync should be ready: %+v", view)
	}
	if view.GeometryState != SharedStateUnavailable {
		t.Fatalf("engineering sync must not imply a full geometry scan: %+v", view)
	}
	if view.QualityState != SharedStateUnavailable {
		t.Fatalf("engineering sync must not imply quality detection: %+v", view)
	}
}

func TestSharedDataViewMarksMeshSwitchStaleWithoutDroppingPayload(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "Mesh-B",
		EngineeringSummary: map[string]any{"project": "E:/sim/model"}, EngineeringSystem: "Mesh-A",
		Geometry: &Snapshot{Project: "E:/sim/model", Fingerprint: "geo-A"}, GeometrySystem: "Mesh-A",
		QualitySummary: map[string]any{"project": "E:/sim/model", "quality": map[string]any{"skewness": map[string]any{"max": 0.9}}},
		QualityPresent: true, QualitySystem: "Mesh-A",
	})
	if view.EngineeringState != SharedStateStale || view.GeometryState != SharedStateStale || view.QualityState != SharedStateStale {
		t.Fatalf("all Mesh-A data must be stale under Mesh-B: %+v", view)
	}
	if view.Project != "E:/sim/model" {
		t.Fatal("stale data should remain visible for traceability")
	}
}

func TestSharedDataViewMarksGeometryAndQualityFingerprintMismatch(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		Geometry: &Snapshot{Project: "E:/sim/model", Fingerprint: "geo-new"}, GeometrySystem: "FFF",
		QualitySummary: map[string]any{"project": "E:/sim/model", "geometry_fingerprint": "geo-old", "quality": map[string]any{"skewness": map[string]any{"max": 0.8}}},
		QualityPresent: true, QualitySystem: "FFF",
	})
	if view.QualityState != SharedStateStale {
		t.Fatalf("quality result with a different geometry fingerprint must be stale: %+v", view)
	}
}

func TestSharedDataViewExposesPartialQualityInsteadOfInventingMissingMetrics(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		QualitySummary: map[string]any{"quality": map[string]any{"skewness": map[string]any{"max": 0.8}}},
		QualityPresent: true, QualitySystem: "FFF",
	})
	if view.QualityState != SharedStatePartial {
		t.Fatalf("missing orthogonal-quality data should be marked partial: %+v", view)
	}
	if view.OQMin != "" {
		t.Fatalf("missing metric must stay missing, got %q", view.OQMin)
	}
}

func TestSharedDataViewPrioritizesRunningAndFailureStates(t *testing.T) {
	loading := deriveSharedDataView(SharedDataInput{Connected: true, Busy: true, ActivePhase: "sync_status"})
	if loading.EngineeringState != SharedStateLoading {
		t.Fatalf("sync should show loading: %+v", loading)
	}
	failed := deriveSharedDataView(SharedDataInput{EngineeringError: "transport error"})
	if failed.EngineeringState != SharedStateFailed {
		t.Fatalf("sync failure should be explicit: %+v", failed)
	}
}

func TestSharedDataViewRejectsMeshSignatureMismatch(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		EngineeringSummary: map[string]any{"project": "E:/sim/model", "mesh_signature": "mesh-B"},
		EngineeringSystem:  "FFF",
		QualitySummary:     map[string]any{"project": "E:/sim/model", "mesh_signature": "mesh-A", "quality": map[string]any{"skewness": map[string]any{"max": 0.9}}},
		QualityPresent:     true, QualitySystem: "FFF",
	})
	if view.QualityState != SharedStateStale {
		t.Fatalf("quality result with a different Mesh signature must be stale: %+v", view)
	}
}

func TestFullGeometryScanCanStartBeforeAnySnapshot(t *testing.T) {
	availability := workflowAvailability{Ready: true, Connected: true, Ack: true, Selected: true}
	if availability.Scanned {
		t.Fatal("test precondition: no geometry snapshot should exist")
	}
	if !availability.Allow("scan") {
		t.Fatal("the full geometry scan must be enabled before the first Snapshot")
	}
}

func TestSharedDataViewTreatsWindowsProjectPathsCaseAndSlashInsensitive(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		EngineeringSummary: map[string]any{"project": `E:\Sim\Model.wbpj`}, EngineeringSystem: "FFF",
		Geometry: &Snapshot{Project: `e:/sim/model.wbpj`, Fingerprint: "geo-A"}, GeometrySystem: "FFF",
		QualitySummary: map[string]any{"project": `E:/SIM/model.wbpj`, "quality": map[string]any{"skewness": map[string]any{"max": 0.9}, "orthogonal_quality": map[string]any{"min": 0.1}}},
		QualityPresent: true, QualitySystem: "FFF",
	})
	if view.GeometryState != SharedStateReady || view.QualityState != SharedStateReady {
		t.Fatalf("Windows path case/slash differences must not falsely stale identical project data: %+v", view)
	}
}

func TestSnapshotRootMeshSignatureSurvivesJSON(t *testing.T) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(`{"project":"E:/model","mesh_signature":"mesh-new","mesh":{"Elements":"1200"}}`), &snap); err != nil {
		t.Fatal(err)
	}
	if snapshotMeshSignature(&snap) != "mesh-new" {
		t.Fatalf("root mesh_signature lost after decoding: %+v", snap)
	}
	legacy := &Snapshot{Mesh: map[string]any{"mesh_signature": "mesh-legacy"}}
	if snapshotMeshSignature(legacy) != "mesh-legacy" {
		t.Fatal("legacy nested mesh_signature not supported")
	}
}

func TestSharedDataViewComparesRootMeshSignature(t *testing.T) {
	input := SharedDataInput{
		Connected: true, CurrentSystem: "FFF",
		Geometry: &Snapshot{Project: "E:/model", Fingerprint: "g", MeshSignature: "new"}, GeometrySystem: "FFF",
		QualitySummary: map[string]any{"project": "E:/model", "mesh_signature": "old", "quality": map[string]any{"skewness": map[string]any{"max": 0.9}, "orthogonal_quality": map[string]any{"min": 0.1}}},
		QualityPresent: true, QualitySystem: "FFF",
	}
	if got := deriveSharedDataView(input).QualityState; got != SharedStateStale {
		t.Fatalf("different root mesh signatures should stale quality, got %s", got)
	}
	input.Geometry.MeshSignature = "old"
	if got := deriveSharedDataView(input).QualityState; got != SharedStateReady {
		t.Fatalf("matching root mesh signatures should be current, got %s", got)
	}
}

func TestSharedDataViewNoMeshIsExplicitlyUnmeasured(t *testing.T) {
	view := deriveSharedDataView(SharedDataInput{
		Connected: true, CurrentSystem: "FFF", QualityPresent: true, QualitySystem: "FFF",
		QualitySummary: map[string]any{"project": "E:/model", "mesh_elements": 0, "quality_status": "not_measured", "quality_gate": "no_mesh"},
	})
	if view.QualityState != SharedStatePartial || view.QualityMessage != "无可用网格，质量尚未测量" || view.SkewMax != "" || view.OQMin != "" {
		t.Fatalf("no-mesh quality must not keep old values or invent metrics: %+v", view)
	}
}
