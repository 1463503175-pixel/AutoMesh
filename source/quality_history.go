package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type QualityMetricCount struct {
	Name        string
	Failed      int64
	Warning     int64
	Available   bool
	Unavailable string
}

type QualitySnapshot struct {
	JobID                  string
	Phase                  string
	Status                 string
	CompletedAt            time.Time
	Project                string
	MeshSystem             string
	GeometryFingerprint    string
	GeometryQuickSignature string
	MeshSignature          string
	QualityDefinition      string
	MeshElements           int64
	Nodes                  int64
	SkewMin                *float64
	SkewMean               *float64
	SkewMax                *float64
	OQMin                  *float64
	OQMean                 *float64
	OQMax                  *float64
	EQMin                  *float64
	EQMean                 *float64
	EQMax                  *float64
	SkewSource             string
	OQSource               string
	EQSource               string
	Worksheet              []QualityMetricCount
	Source                 string
	HasSkewness            bool
	HasOrthogonal          bool
	HasElementQuality      bool
	SkewDistribution       *SkewnessDistribution
	SkewThresholds         *SkewThresholdStats
	OQBound                *OrthogonalQualityBound
	GeometryVerified       bool
}

func mapAt(v any, key string) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func numberAt(m map[string]any, key string) *float64 {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case int:
		n = float64(x)
	case int64:
		n = float64(x)
	case json.Number:
		parsed, err := x.Float64()
		if err != nil {
			return nil
		}
		n = parsed
	default:
		return nil
	}
	return &n
}

func int64At(m map[string]any, key string) int64 {
	v := numberAt(m, key)
	if v == nil || *v < 0 {
		return 0
	}
	return int64(*v)
}

func stringAt(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

func parseQualitySnapshot(result Result, modTime time.Time) (QualitySnapshot, bool) {
	if result.Status != "ok" || result.JobID == "" {
		return QualitySnapshot{}, false
	}
	var summary map[string]any
	if len(result.Summary) == 0 || json.Unmarshal(result.Summary, &summary) != nil {
		return QualitySnapshot{}, false
	}
	quality := mapAt(summary["quality"], "")
	if len(quality) == 0 {
		return QualitySnapshot{}, false
	}
	q := QualitySnapshot{
		JobID: result.JobID, Phase: result.Phase, Status: result.Status,
		Project:                stringAt(summary, "project"),
		MeshSystem:             stringAt(summary, "mesh_system"),
		GeometryFingerprint:    stringAt(summary, "geometry_fingerprint"),
		GeometryQuickSignature: stringAt(summary, "geometry_quick_signature"),
		MeshSignature:          stringAt(summary, "mesh_signature"),
		QualityDefinition:      stringAt(summary, "quality_definition"),
		MeshElements:           int64At(summary, "mesh_elements"),
		Source:                 stringAt(summary, "quality_source"),
		GeometryVerified:       summary["geometry_identity_verified"] == true,
	}
	q.SkewMin = numberAt(mapAt(quality["skewness"], ""), "min")
	q.SkewMean = numberAt(mapAt(quality["skewness"], ""), "mean")
	q.SkewMax = numberAt(mapAt(quality["skewness"], ""), "max")
	q.OQMin = numberAt(mapAt(quality["orthogonal_quality"], ""), "min")
	q.OQMean = numberAt(mapAt(quality["orthogonal_quality"], ""), "mean")
	q.OQMax = numberAt(mapAt(quality["orthogonal_quality"], ""), "max")
	q.EQMin = numberAt(mapAt(quality["element_quality"], ""), "min")
	q.EQMean = numberAt(mapAt(quality["element_quality"], ""), "mean")
	q.EQMax = numberAt(mapAt(quality["element_quality"], ""), "max")
	if q.MeshElements == 0 {
		q.MeshElements = int64At(quality, "elements")
	}
	q.SkewSource = stringAt(mapAt(quality["skewness"], ""), "source")
	q.OQSource = stringAt(mapAt(quality["orthogonal_quality"], ""), "source")
	q.EQSource = stringAt(mapAt(quality["element_quality"], ""), "source")
	q.HasSkewness = q.SkewMin != nil && q.SkewMean != nil && q.SkewMax != nil
	q.HasOrthogonal = q.OQMin != nil && q.OQMean != nil && q.OQMax != nil
	q.HasElementQuality = q.EQMin != nil && q.EQMean != nil && q.EQMax != nil
	// Exact percentiles, histogram and region counts require complete,
	// native per-element data. Old receipts have no such field.
	if rawDistribution, exists := quality["skewness_distribution"]; exists {
		if encoded, err := json.Marshal(rawDistribution); err == nil {
			var d SkewnessDistribution
			if json.Unmarshal(encoded, &d) == nil && validateSkewnessDistribution(d, q.MeshElements) == nil && q.HasSkewness {
				q.SkewDistribution = &d
			}
		}
	}
	if raw, ok := quality["skewness_threshold_statistics"]; ok && q.HasSkewness {
		if data, err := json.Marshal(raw); err == nil {
			var stats SkewThresholdStats
			if json.Unmarshal(data, &stats) == nil && validateSkewThresholdStats(stats, q.MeshElements, q.SkewMax) == nil {
				q.SkewThresholds = &stats
			}
		}
	}
	if raw, ok := quality["orthogonal_quality_bound"]; ok {
		if data, err := json.Marshal(raw); err == nil {
			var bounds OrthogonalQualityBound
			if json.Unmarshal(data, &bounds) == nil && validateOrthogonalQualityBound(bounds, q.MeshElements) == nil {
				q.OQBound = &bounds
			}
		}
	}
	q.Nodes = int64At(quality, "nodes")
	if q.Source == "" {
		q.Source = "ANSYS ACT quality metrics"
	}
	if raw, ok := summary["worksheet_diagnostics"].(map[string]any); ok {
		if rows, ok := raw["metrics"].([]any); ok {
			for _, item := range rows {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				row := QualityMetricCount{Name: stringAt(m, "name")}
				if _, exists := m["failed_count"]; exists {
					row.Failed = int64At(m, "failed_count")
					row.Warning = int64At(m, "warning_count")
					row.Available = true
				} else {
					row.Unavailable = stringAt(m, "error")
				}
				q.Worksheet = append(q.Worksheet, row)
			}
		}
	}
	if result.CompletedAtUnix > 0 {
		seconds := int64(result.CompletedAtUnix)
		nanos := int64((result.CompletedAtUnix - float64(seconds)) * float64(time.Second))
		q.CompletedAt = time.Unix(seconds, nanos)
	} else if result.CompletedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, result.CompletedAt); err == nil {
			q.CompletedAt = parsed
		} else if parsed, err := time.Parse("2006-01-02T15:04:05-0700", result.CompletedAt); err == nil {
			q.CompletedAt = parsed
		}
	}
	if q.CompletedAt.IsZero() {
		q.CompletedAt = modTime
	}
	return q, q.HasSkewness || q.HasOrthogonal || q.HasElementQuality
}

// LoadQualityHistory reuses immutable per-job receipts. It never contacts
// Workbench/ANSYS and does not copy large result files into a second archive.
func (w Workspace) LoadQualityHistory() ([]QualitySnapshot, error) {
	matches, err := filepath.Glob(w.path("job_*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]QualitySnapshot, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "job_") || !strings.HasSuffix(base, ".json") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(base, "job_"), ".json")
		if len(id) != 32 {
			continue
		}
		result, readErr := readJSON[Result](path)
		if readErr != nil {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		q, ok := parseQualitySnapshot(result, info.ModTime())
		if ok {
			out = append(out, q)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CompletedAt.Equal(out[j].CompletedAt) {
			return out[i].JobID < out[j].JobID
		}
		return out[i].CompletedAt.Before(out[j].CompletedAt)
	})
	return out, nil
}

func qualitySnapshotsComparable(before, after QualitySnapshot) (bool, string) {
	beforeProject := strings.TrimRight(strings.ReplaceAll(filepath.Clean(before.Project), "\\", "/"), "/")
	afterProject := strings.TrimRight(strings.ReplaceAll(filepath.Clean(after.Project), "\\", "/"), "/")
	if before.Project == "" || after.Project == "" || !strings.EqualFold(beforeProject, afterProject) {
		return false, "工程标识不同，不能作为同一工程的严格前后对比"
	}
	if before.MeshSystem == "" || after.MeshSystem == "" || !strings.EqualFold(before.MeshSystem, after.MeshSystem) {
		return false, "Mesh 系统标识缺失或不同；不能进行严格前后对比"
	}
	if !before.GeometryVerified || !after.GeometryVerified {
		return false, "至少一条快照没有完整几何身份核验；只能并列查看，不能判定为同一几何的优化效果"
	}
	if before.GeometryFingerprint == "" || after.GeometryFingerprint == "" || before.GeometryFingerprint != after.GeometryFingerprint {
		return false, "几何指纹缺失或不同；只能并列查看，不能判定为同一几何的优化效果"
	}
	if before.QualityDefinition == "" || after.QualityDefinition == "" || before.QualityDefinition != after.QualityDefinition {
		return false, "质量定义版本不一致或缺失；不能进行严格指标对比"
	}
	if before.SkewSource == "" || after.SkewSource == "" || before.SkewSource != after.SkewSource {
		return false, "偏度指标采集来源缺失或不一致；不能进行严格前后对比"
	}
	return true, "同一工程、同一几何指纹、同一质量定义和偏度采集来源；网格规模变化会单独列出"
}

// qualitySnapshotsReferenceComparable permits a *labelled arithmetic difference*
// when full geometry identity is missing, but there is no explicit evidence of
// conflicting geometry. It never replaces the strict verification gate above:
// reference deltas must not be reported as proven mesh-optimization effects.
func qualitySnapshotsReferenceComparable(before, after QualitySnapshot) (bool, string) {
	if before.JobID == "" || before.JobID == after.JobID {
		return false, "请选择两次不同的质量检测记录"
	}
	beforeProject := strings.TrimRight(strings.ReplaceAll(filepath.Clean(before.Project), "\\", "/"), "/")
	afterProject := strings.TrimRight(strings.ReplaceAll(filepath.Clean(after.Project), "\\", "/"), "/")
	if before.Project == "" || after.Project == "" || !strings.EqualFold(beforeProject, afterProject) {
		return false, "工程不同，不能展示参考差值"
	}
	if before.MeshSystem == "" || after.MeshSystem == "" || !strings.EqualFold(before.MeshSystem, after.MeshSystem) {
		return false, "Mesh 系统不同，不能展示参考差值"
	}
	if before.QualityDefinition == "" || before.QualityDefinition != after.QualityDefinition ||
		before.SkewSource == "" || before.SkewSource != after.SkewSource {
		return false, "质量定义或偏度来源不一致，不能展示参考差值"
	}
	if before.GeometryFingerprint != "" && after.GeometryFingerprint != "" && before.GeometryFingerprint != after.GeometryFingerprint {
		return false, "已记录的完整几何指纹不同，不能展示参考差值"
	}
	if before.GeometryQuickSignature != "" && after.GeometryQuickSignature != "" &&
		before.GeometryQuickSignature != after.GeometryQuickSignature {
		return false, "已记录的轻量几何指纹不同，不能展示参考差值"
	}
	if before.GeometryQuickSignature != "" && before.GeometryQuickSignature == after.GeometryQuickSignature {
		return true, "轻量几何签名一致，但完整几何未核验；下列仅为参考数值差，不是已验证的优化效果"
	}
	return true, "完整几何身份与轻量几何签名未充分核验；下列仅为参考数值差，不是已验证的优化效果"
}
