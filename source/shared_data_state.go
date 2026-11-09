package main

import (
	"strconv"
	"strings"
	"time"
)

// SharedDataState is a presentation state, not a second copy of the underlying
// engineering/geometry/quality payload. The payload remains owned by the
// application's single state object; this projection gives every page the
// same freshness and identity interpretation.
type SharedDataState string

const (
	SharedStateUnavailable SharedDataState = "unavailable"
	SharedStateLoading     SharedDataState = "loading"
	SharedStateReady       SharedDataState = "ready"
	SharedStateStale       SharedDataState = "stale"
	SharedStateFailed      SharedDataState = "failed"
	SharedStatePartial     SharedDataState = "partial"
)

type SharedDataInput struct {
	Connected       bool
	Connecting      bool
	ConnectionError string
	InitError       string
	Busy            bool
	ActivePhase     string
	CurrentSystem   string

	EngineeringSummary map[string]any
	EngineeringSystem  string
	EngineeringAt      time.Time
	EngineeringStale   bool
	EngineeringError   string

	Geometry          *Snapshot
	GeometrySystem    string
	GeometryScannedAt time.Time
	GeometryStale     bool
	GeometryError     string

	QualitySummary map[string]any
	QualityPresent bool
	QualitySystem  string
	QualityAt      time.Time
	QualityStale   bool
	QualityError   string
}

type SharedDataView struct {
	ConnectionState  SharedDataState
	EngineeringState SharedDataState
	GeometryState    SharedDataState
	QualityState     SharedDataState

	Project       string
	MeshSystem    string
	EngineeringAt time.Time
	GeometryAt    time.Time
	QualityAt     time.Time

	BodyCount    string
	FaceCount    string
	NamedCount   string
	ControlCount string
	MeshElements string
	SkewMax      string
	OQMin        string

	EngineeringMessage string
	GeometryMessage    string
	QualityMessage     string
}

func sharedString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return ""
	}
}

func sameSharedProjectIdentity(a, b string) bool {
	normalize := func(value string) string {
		value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
		value = strings.TrimRight(value, "/")
		return strings.ToLower(value)
	}
	return normalize(a) != "" && normalize(a) == normalize(b)
}

func sharedNestedMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	x, _ := m[key].(map[string]any)
	return x
}

// snapshotMeshSignature reads the typed root-level ACT field. Older saved
// snapshots may contain the signature inside the Mesh map instead.
func snapshotMeshSignature(s *Snapshot) string {
	if s == nil {
		return ""
	}
	if s.MeshSignature != "" {
		return s.MeshSignature
	}
	return sharedString(s.Mesh, "mesh_signature")
}

func deriveSharedDataView(in SharedDataInput) SharedDataView {
	v := SharedDataView{MeshSystem: in.CurrentSystem, EngineeringAt: in.EngineeringAt,
		GeometryAt: in.GeometryScannedAt, QualityAt: in.QualityAt}

	switch {
	case in.Connected:
		v.ConnectionState = SharedStateReady
	case in.Connecting || (in.Busy && in.ActivePhase == "connect"):
		v.ConnectionState = SharedStateLoading
	case in.ConnectionError != "" || in.InitError != "":
		v.ConnectionState = SharedStateFailed
	default:
		v.ConnectionState = SharedStateUnavailable
	}

	v.Project = sharedString(in.EngineeringSummary, "project")
	if v.Project == "" && in.Geometry != nil {
		v.Project = in.Geometry.Project
	}
	if v.Project == "" {
		v.Project = sharedString(in.QualitySummary, "project")
	}
	v.BodyCount = sharedString(in.EngineeringSummary, "body_count")
	v.FaceCount = sharedString(in.EngineeringSummary, "face_count")
	v.NamedCount = sharedString(in.EngineeringSummary, "named_count")
	v.ControlCount = sharedString(in.EngineeringSummary, "control_count")
	v.MeshElements = sharedString(in.QualitySummary, "mesh_elements")
	quality := sharedNestedMap(in.QualitySummary, "quality")
	skew := sharedNestedMap(quality, "skewness")
	orthogonal := sharedNestedMap(quality, "orthogonal_quality")
	v.SkewMax = sharedScalarText(skew, "max")
	v.OQMin = sharedScalarText(orthogonal, "min")

	switch {
	case in.Busy && in.ActivePhase == "sync_status":
		v.EngineeringState = SharedStateLoading
	case in.EngineeringError != "":
		v.EngineeringState = SharedStateFailed
	case len(in.EngineeringSummary) == 0:
		v.EngineeringState = SharedStateUnavailable
	case in.EngineeringStale || in.CurrentSystem == "" || in.EngineeringSystem == "" || in.EngineeringSystem != in.CurrentSystem:
		v.EngineeringState = SharedStateStale
	default:
		v.EngineeringState = SharedStateReady
	}

	switch {
	case in.Busy && in.ActivePhase == "inspect":
		v.GeometryState = SharedStateLoading
	case in.GeometryError != "":
		v.GeometryState = SharedStateFailed
	case in.GeometryStale:
		v.GeometryState = SharedStateStale
	case in.Geometry == nil || in.GeometrySystem == "":
		v.GeometryState = SharedStateUnavailable
	case in.CurrentSystem == "" || in.GeometrySystem != in.CurrentSystem:
		v.GeometryState = SharedStateStale
	case v.Project != "" && in.Geometry.Project != "" && !sameSharedProjectIdentity(v.Project, in.Geometry.Project):
		v.GeometryState = SharedStateStale
	default:
		v.GeometryState = SharedStateReady
	}

	switch {
	case in.Busy && (in.ActivePhase == "quality_check" || in.ActivePhase == "diagnose"):
		v.QualityState = SharedStateLoading
	case in.QualityError != "":
		v.QualityState = SharedStateFailed
	case !in.QualityPresent || len(in.QualitySummary) == 0:
		v.QualityState = SharedStateUnavailable
	case in.QualityStale || in.CurrentSystem == "" || in.QualitySystem == "" || in.QualitySystem != in.CurrentSystem:
		v.QualityState = SharedStateStale
	case v.Project != "" && sharedString(in.QualitySummary, "project") != "" && !sameSharedProjectIdentity(v.Project, sharedString(in.QualitySummary, "project")):
		v.QualityState = SharedStateStale
	case in.Geometry != nil && in.GeometrySystem == in.CurrentSystem && sharedString(in.QualitySummary, "geometry_fingerprint") != "" && in.Geometry.Fingerprint != "" && sharedString(in.QualitySummary, "geometry_fingerprint") != in.Geometry.Fingerprint:
		v.QualityState = SharedStateStale
	case in.Geometry != nil && in.GeometrySystem == in.CurrentSystem && sharedString(in.QualitySummary, "geometry_quick_signature") != "" && in.Geometry.GeometryQuickSignature != "" && sharedString(in.QualitySummary, "geometry_quick_signature") != in.Geometry.GeometryQuickSignature:
		v.QualityState = SharedStateStale
	case in.Geometry != nil && in.GeometrySystem == in.CurrentSystem && sharedString(in.QualitySummary, "mesh_signature") != "" && snapshotMeshSignature(in.Geometry) != "" && sharedString(in.QualitySummary, "mesh_signature") != snapshotMeshSignature(in.Geometry):
		v.QualityState = SharedStateStale
	case in.EngineeringSystem == in.CurrentSystem && in.EngineeringSystem != "" && sharedString(in.EngineeringSummary, "mesh_signature") != "" && sharedString(in.QualitySummary, "mesh_signature") != "" && sharedString(in.EngineeringSummary, "mesh_signature") != sharedString(in.QualitySummary, "mesh_signature"):
		v.QualityState = SharedStateStale
	case sharedString(in.QualitySummary, "quality_status") == "not_measured" && sharedString(in.QualitySummary, "quality_gate") == "no_mesh":
		v.QualityState = SharedStatePartial
	case len(skew) == 0 && len(orthogonal) == 0:
		v.QualityState = SharedStatePartial
	case len(skew) == 0 || len(orthogonal) == 0:
		v.QualityState = SharedStatePartial
	default:
		v.QualityState = SharedStateReady
	}

	v.EngineeringMessage = sharedStateMessage(v.EngineeringState, in.EngineeringAt)
	v.GeometryMessage = sharedStateMessage(v.GeometryState, in.GeometryScannedAt)
	v.QualityMessage = sharedStateMessage(v.QualityState, in.QualityAt)
	if v.QualityState == SharedStatePartial && sharedString(in.QualitySummary, "quality_status") == "not_measured" && sharedString(in.QualitySummary, "quality_gate") == "no_mesh" {
		v.QualityMessage = "无可用网格，质量尚未测量"
	}
	return v
}

func sharedScalarText(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', 4, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return x
	default:
		return ""
	}
}

func sharedStateMessage(state SharedDataState, at time.Time) string {
	switch state {
	case SharedStateLoading:
		return "读取中"
	case SharedStateReady:
		if !at.IsZero() {
			return "有效 · " + at.Format("15:04:05")
		}
		return "有效"
	case SharedStateStale:
		return "数据已过期，需重新读取"
	case SharedStateFailed:
		return "读取失败，保留上次数据"
	case SharedStatePartial:
		return "已读取，部分指标未取得"
	default:
		return "尚未读取"
	}
}
