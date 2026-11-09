package main

import (
	"math"
	"sort"
)

// QualityTrendPoint contains only values present in validated, immutable
// receipts. Nil values are intentionally missing, never zero-filled.
type QualityTrendPoint struct {
	Snapshot              QualitySnapshot
	SkewMaximum           *float64
	SkewAverage           *float64
	OrthogonalMinimum     *float64
	ElementQualityMinimum *float64
	ElementQualityAverage *float64
	P95                   *float64
	High090Fraction       *float64
	High095Fraction       *float64
}

func fraction(count, total int64) *float64 {
	if total <= 0 || count < 0 || count > total {
		return nil
	}
	value := float64(count) / float64(total)
	return &value
}

// CompatibleQualityTrend does not blend multiple geometries or metric
// definitions. Distribution-only metrics additionally require the same native
// producer as the anchor. A different mesh generation is allowed.
func CompatibleQualityTrend(history []QualitySnapshot, anchor QualitySnapshot) []QualityTrendPoint {
	points := make([]QualityTrendPoint, 0, len(history))
	for _, q := range history {
		if ok, _ := qualitySnapshotsComparable(q, anchor); !ok {
			continue
		}
		point := QualityTrendPoint{Snapshot: q, SkewMaximum: q.SkewMax, SkewAverage: q.SkewMean}
		if q.OQMin != nil && anchor.OQSource != "" && q.OQSource == anchor.OQSource {
			point.OrthogonalMinimum = q.OQMin
		}
		if q.EQMin != nil && anchor.EQSource != "" && q.EQSource == anchor.EQSource {
			point.ElementQualityMinimum = q.EQMin
			point.ElementQualityAverage = q.EQMean
		}
		if q.SkewDistribution != nil && anchor.SkewDistribution != nil &&
			q.SkewDistribution.Source == anchor.SkewDistribution.Source {
			value := q.SkewDistribution.P95
			point.P95 = &value
			point.High090Fraction = fraction(q.SkewDistribution.Above090, q.MeshElements)
			point.High095Fraction = fraction(q.SkewDistribution.Above095, q.MeshElements)
		}
		points = append(points, point)
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].Snapshot.CompletedAt.Before(points[j].Snapshot.CompletedAt) })
	return points
}

// RegionComparison never interprets a missing Body as zero affected cells.
// Before is nil and Comparable is false when the source or identity differs.
type RegionComparison struct {
	Current      BodySkewStatistics
	Before       *BodySkewStatistics
	Comparable   bool
	DeltaP95     *float64
	DeltaHigh095 *int64
}

func QualityRegionComparisons(before, after QualitySnapshot) []RegionComparison {
	if after.SkewDistribution == nil {
		return nil
	}
	current := after.SkewDistribution.BodyStats
	rows := make([]RegionComparison, 0, len(current))
	for _, body := range current {
		row := RegionComparison{Current: body}
		previous, _, ok := comparableNativeBodyStats(before, after, body.BodyID)
		if ok && before.JobID != after.JobID {
			row.Before = &previous
			row.Comparable = true
			p95Delta := body.P95 - previous.P95
			highDelta := body.Above095 - previous.Above095
			row.DeltaP95 = &p95Delta
			row.DeltaHigh095 = &highDelta
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Current.Above095 == rows[j].Current.Above095 {
			return rows[i].Current.BodyID < rows[j].Current.BodyID
		}
		return rows[i].Current.Above095 > rows[j].Current.Above095
	})
	return rows
}

func metricDifference(before, after *float64, comparable bool) *float64 {
	if !comparable || before == nil || after == nil ||
		math.IsNaN(*before) || math.IsNaN(*after) || math.IsInf(*before, 0) || math.IsInf(*after, 0) {
		return nil
	}
	delta := *after - *before
	return &delta
}
