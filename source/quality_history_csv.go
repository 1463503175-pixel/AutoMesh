package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// buildQualityHistoryCSVRows is pure and testable without Windows. Every
// header must correspond to exactly one cell; missing ACT values are "未取得".
func buildQualityHistoryCSVRows(history []QualitySnapshot) [][]string {
	rows := [][]string{{"job_id", "phase", "completed_at", "project", "mesh_system", "geometry_fingerprint", "geometry_quick_signature", "mesh_signature", "mesh_elements", "nodes", "skew_min", "skew_mean", "skew_max", "oq_min", "oq_mean", "oq_max", "eq_min", "eq_mean", "eq_max", "eq_source", "quality_source", "skew_source", "oq_source", "has_skewness", "has_orthogonal_quality", "has_element_quality", "quality_definition", "geometry_verified", "worksheet_summary", "skew_p95", "skew_above_0_90", "skew_above_0_95", "histogram_json", "body_statistics_json", "skew_distribution_source", "worksheet_gt_090", "worksheet_gt_095", "worksheet_p95_lower", "worksheet_p95_upper", "worksheet_oq_min_lower", "worksheet_oq_min_upper", "worksheet_threshold_source"}}
	for _, q := range history {
		worksheet := make([]string, 0, len(q.Worksheet))
		for _, item := range q.Worksheet {
			if item.Available {
				worksheet = append(worksheet, fmt.Sprintf("%s: failed=%d warning=%d", item.Name, item.Failed, item.Warning))
			} else {
				worksheet = append(worksheet, fmt.Sprintf("%s: unavailable=%s", item.Name, item.Unavailable))
			}
		}
		p95, over90, over95, histogramJSON, bodyJSON, distributionSource := "", "", "", "", "", ""
		if q.SkewDistribution != nil {
			p95 = fmt.Sprintf("%.9g", q.SkewDistribution.P95)
			over90 = strconv.FormatInt(q.SkewDistribution.Above090, 10)
			over95 = strconv.FormatInt(q.SkewDistribution.Above095, 10)
			binsJSON, _ := json.Marshal(q.SkewDistribution.Bins)
			bodiesJSON, _ := json.Marshal(q.SkewDistribution.BodyStats)
			histogramJSON, bodyJSON = string(binsJSON), string(bodiesJSON)
			distributionSource = q.SkewDistribution.Source
		}
		worksheet90, worksheet95, worksheetP95Low, worksheetP95High, worksheetOQLow, worksheetOQHigh, worksheetSource := "", "", "", "", "", "", ""
		if q.SkewThresholds != nil {
			worksheet90 = strconv.FormatInt(q.SkewThresholds.Above090, 10)
			worksheet95 = strconv.FormatInt(q.SkewThresholds.Above095, 10)
			worksheetP95Low = fmt.Sprintf("%.6f", q.SkewThresholds.P95Lower)
			worksheetP95High = fmt.Sprintf("%.6f", q.SkewThresholds.P95Upper)
			worksheetSource = q.SkewThresholds.Source
		}
		if q.OQBound != nil {
			worksheetOQLow = fmt.Sprintf("%.6f", q.OQBound.MinLower)
			worksheetOQHigh = fmt.Sprintf("%.6f", q.OQBound.MinUpper)
			if worksheetSource == "" {
				worksheetSource = q.OQBound.Source
			} else {
				worksheetSource += "; " + q.OQBound.Source
			}
		}
		rows = append(rows, []string{q.JobID, q.Phase, q.CompletedAt.Format(time.RFC3339), q.Project, q.MeshSystem,
			q.GeometryFingerprint, q.GeometryQuickSignature, q.MeshSignature, csvHistoryCount(q.MeshElements), csvHistoryCount(q.Nodes),
			csvHistoryMetric(q.SkewMin), csvHistoryMetric(q.SkewMean), csvHistoryMetric(q.SkewMax),
			csvHistoryMetric(q.OQMin), csvHistoryMetric(q.OQMean), csvHistoryMetric(q.OQMax),
			csvHistoryMetric(q.EQMin), csvHistoryMetric(q.EQMean), csvHistoryMetric(q.EQMax), q.EQSource,
			q.Source, q.SkewSource, q.OQSource,
			strconv.FormatBool(q.HasSkewness), strconv.FormatBool(q.HasOrthogonal), strconv.FormatBool(q.HasElementQuality), q.QualityDefinition,
			strconv.FormatBool(q.GeometryVerified), strings.Join(worksheet, "; "),
			p95, over90, over95, histogramJSON, bodyJSON, distributionSource,
			worksheet90, worksheet95, worksheetP95Low, worksheetP95High, worksheetOQLow, worksheetOQHigh, worksheetSource})
	}
	return rows
}

func csvHistoryCount(n int64) string {
	if n <= 0 {
		return "未取得"
	}
	return strconv.FormatInt(n, 10)
}

func csvHistoryMetric(p *float64) string {
	if p == nil {
		return "未取得"
	}
	return fmt.Sprintf("%.4f", *p)
}
