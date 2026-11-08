package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	qualityHistorySchema = 1
	maxQualityHistory    = 60
)

type QualityMetricSnapshot struct {
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
	Mean   *float64 `json:"mean,omitempty"`
	Source string   `json:"source,omitempty"`
}

type QualityWorksheetSnapshot struct {
	Name    string `json:"name"`
	Failed  *int64 `json:"failed_count,omitempty"`
	Warning *int64 `json:"warning_count,omitempty"`
}

// QualitySnapshot stores only metrics returned by the active ANSYS ACT task.
// Missing values remain nil; they are never replaced with zero or inferred.
type QualitySnapshot struct {
	ID                  string                     `json:"id"`
	JobID               string                     `json:"job_id"`
	CapturedAt          time.Time                  `json:"captured_at"`
	Project             string                     `json:"project"`
	GeometryFingerprint string                     `json:"geometry_fingerprint"`
	MeshElements        int64                      `json:"mesh_elements,omitempty"`
	MeshNodes           int64                      `json:"mesh_nodes,omitempty"`
	Skewness            QualityMetricSnapshot      `json:"skewness"`
	OrthogonalQuality   *QualityMetricSnapshot     `json:"orthogonal_quality,omitempty"`
	Worksheet           []QualityWorksheetSnapshot `json:"worksheet,omitempty"`
	QualityGate         string                     `json:"quality_gate,omitempty"`
	QualityStatus       string                     `json:"quality_status,omitempty"`
	Source              string                     `json:"source,omitempty"`
}

type qualityHistoryFile struct {
	Schema int               `json:"schema"`
	Items  []QualitySnapshot `json:"items"`
}

func qualityObject(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func qualityText(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func qualityNumber(v any) (*float64, bool) {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case float32:
		n = float64(x)
	case int:
		n = float64(x)
	case int64:
		n = float64(x)
	case json.Number:
		var err error
		n, err = x.Float64()
		if err != nil {
			return nil, false
		}
	case string:
		var err error
		n, err = strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return nil, false
		}
	default:
		return nil, false
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil, false
	}
	return &n, true
}

func qualityCount(v any) (*int64, bool) {
	n, ok := qualityNumber(v)
	if !ok || *n < 0 || math.Trunc(*n) != *n || *n > math.MaxInt64 {
		return nil, false
	}
	value := int64(*n)
	return &value, true
}

func qualityMetricFromMap(m map[string]any) QualityMetricSnapshot {
	min, _ := qualityNumber(m["min"])
	max, _ := qualityNumber(m["max"])
	mean, _ := qualityNumber(m["mean"])
	return QualityMetricSnapshot{
		Min: min, Max: max, Mean: mean,
		Source: qualityText(m["source"]),
	}
}

// qualitySnapshotFromSummary accepts only a completed result with actual
// quality data. Project/fingerprint fallbacks come from the verified scan
// snapshot, not from a guessed mesh identity.
func qualitySnapshotFromSummary(result Result, summary map[string]any, project, fingerprint string, capturedAt time.Time) (QualitySnapshot, bool) {
	if result.JobID == "" || result.Error != "" {
		return QualitySnapshot{}, false
	}
	quality := qualityObject(summary["quality"])
	if len(quality) == 0 {
		return QualitySnapshot{}, false
	}

	if p := qualityText(summary["project"]); p != "" {
		project = p
	}
	if f := qualityText(summary["geometry_fingerprint"]); f != "" {
		fingerprint = f
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now()
	}

	out := QualitySnapshot{
		ID:                  result.JobID,
		JobID:               result.JobID,
		CapturedAt:          capturedAt,
		Project:             project,
		GeometryFingerprint: fingerprint,
		Skewness:            qualityMetricFromMap(qualityObject(quality["skewness"])),
		QualityGate:         qualityText(summary["quality_gate"]),
		QualityStatus:       qualityText(summary["quality_status"]),
		Source:              "ANSYS ACT",
	}

	if n, ok := qualityCount(summary["mesh_elements"]); ok {
		out.MeshElements = *n
	} else if n, ok := qualityCount(quality["elements"]); ok {
		out.MeshElements = *n
	}
	if n, ok := qualityCount(quality["mesh_nodes"]); ok {
		out.MeshNodes = *n
	} else if n, ok := qualityCount(quality["nodes"]); ok {
		out.MeshNodes = *n
	}

	oq := qualityObject(quality["orthogonal_quality"])
	if len(oq) > 0 {
		metric := qualityMetricFromMap(oq)
		out.OrthogonalQuality = &metric
	}
	if out.Skewness.Source != "" {
		out.Source = out.Skewness.Source
	} else if out.OrthogonalQuality != nil && out.OrthogonalQuality.Source != "" {
		out.Source = out.OrthogonalQuality.Source
	}

	worksheet := qualityObject(summary["worksheet_diagnostics"])
	if rows, ok := worksheet["metrics"].([]any); ok {
		for _, raw := range rows {
			row := qualityObject(raw)
			name := qualityText(row["name"])
			if name == "" {
				continue
			}
			failed, hasFailed := qualityCount(row["failed_count"])
			warning, hasWarning := qualityCount(row["warning_count"])
			if !hasFailed && !hasWarning {
				continue
			}
			out.Worksheet = append(out.Worksheet, QualityWorksheetSnapshot{
				Name: name, Failed: failed, Warning: warning,
			})
		}
	}
	return out, true
}

func normalizeQualityProject(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "/", "\\"))
}

// Snapshots are comparable only when both the project and geometry identity
// match. A mesh may change between rounds; that is the intended comparison.
func qualitySnapshotsComparable(before, after QualitySnapshot) bool {
	if before.Project == "" || after.Project == "" ||
		before.GeometryFingerprint == "" || after.GeometryFingerprint == "" {
		return false
	}
	return normalizeQualityProject(before.Project) == normalizeQualityProject(after.Project) &&
		before.GeometryFingerprint == after.GeometryFingerprint
}

func (w Workspace) LoadQualityHistory() ([]QualitySnapshot, error) {
	file, err := readJSON[qualityHistoryFile](w.path("quality_history.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if file.Schema != 0 && file.Schema != qualityHistorySchema {
		return nil, fmt.Errorf("unsupported quality history schema %d", file.Schema)
	}
	items := make([]QualitySnapshot, 0, len(file.Items))
	for _, item := range file.Items {
		if item.ID == "" || item.CapturedAt.IsZero() {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CapturedAt.Before(items[j].CapturedAt)
	})
	return items, nil
}

func (w Workspace) SaveQualitySnapshot(snapshot QualitySnapshot) ([]QualitySnapshot, error) {
	if snapshot.ID == "" || snapshot.CapturedAt.IsZero() {
		return nil, fmt.Errorf("quality snapshot requires an ID and capture time")
	}
	items, err := w.LoadQualityHistory()
	if err != nil {
		return nil, err
	}
	replaced := false
	for i := range items {
		if items[i].ID == snapshot.ID {
			items[i] = snapshot
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, snapshot)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CapturedAt.Before(items[j].CapturedAt)
	})
	if len(items) > maxQualityHistory {
		items = append([]QualitySnapshot(nil), items[len(items)-maxQualityHistory:]...)
	}
	file := qualityHistoryFile{Schema: qualityHistorySchema, Items: items}
	if err := writeJSON(w.path("quality_history.json"), file); err != nil {
		return nil, err
	}
	return items, nil
}

func qualityCSVNumber(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func qualityCSVCount(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func (w Workspace) ExportQualityHistoryCSV(filename string, items []QualitySnapshot) error {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	header := []string{
		"captured_at", "job_id", "project", "geometry_fingerprint",
		"mesh_elements", "mesh_nodes",
		"skewness_min", "skewness_max", "skewness_mean", "skewness_source",
		"orthogonal_quality_min", "orthogonal_quality_max", "orthogonal_quality_mean",
		"orthogonal_quality_source", "quality_gate", "quality_status", "worksheet_metrics",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, item := range items {
		oq := QualityMetricSnapshot{}
		if item.OrthogonalQuality != nil {
			oq = *item.OrthogonalQuality
		}
		worksheetParts := make([]string, 0, len(item.Worksheet))
		for _, metric := range item.Worksheet {
			worksheetParts = append(worksheetParts,
				fmt.Sprintf("%s:failed=%s;warning=%s", metric.Name, qualityCSVCount(metric.Failed), qualityCSVCount(metric.Warning)))
		}
		row := []string{
			item.CapturedAt.Format(time.RFC3339), item.JobID, item.Project, item.GeometryFingerprint,
			strconv.FormatInt(item.MeshElements, 10), strconv.FormatInt(item.MeshNodes, 10),
			qualityCSVNumber(item.Skewness.Min), qualityCSVNumber(item.Skewness.Max),
			qualityCSVNumber(item.Skewness.Mean), item.Skewness.Source,
			qualityCSVNumber(oq.Min), qualityCSVNumber(oq.Max), qualityCSVNumber(oq.Mean),
			oq.Source, item.QualityGate, item.QualityStatus, strings.Join(worksheetParts, " | "),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return writeAtomic(filename, buf.Bytes())
}
