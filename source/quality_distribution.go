package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// NativeSkewSample is a single REAL, native-metric value, not a shape proxy.
// Only a verified per-element Skewness producer is allowed to provide these.
// The current Meshing ACT integration does not yet expose such a producer.
type NativeSkewSample struct {
	ElementID int64
	BodyID    int64
	Skewness  float64
}

type SkewHistogramBin struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
	Count int64   `json:"count"`
}

type BodySkewStatistics struct {
	BodyID   int64   `json:"body_id"`
	Count    int64   `json:"count"`
	Mean     float64 `json:"mean"`
	Max      float64 `json:"max"`
	P95      float64 `json:"p95"`
	Above090 int64   `json:"above_0_90"`
	Above095 int64   `json:"above_0_95"`
}

type SkewnessDistribution struct {
	Status      string               `json:"status"`
	Method      string               `json:"method"`
	Source      string               `json:"source"`
	SampleCount int64                `json:"sample_count"`
	P95         float64              `json:"p95"`
	Above090    int64                `json:"above_0_90"`
	Above095    int64                `json:"above_0_95"`
	Bins        []SkewHistogramBin   `json:"bins"`
	BodyStats   []BodySkewStatistics `json:"body_stats,omitempty"`
}

// Worksheet threshold readings are NOT a complete per-element distribution.
// Exact threshold counts and bounded P95 must remain distinguishable from a
// verified, exact native_per_element distribution.
type SkewThresholdStats struct {
	Status               string  `json:"status"`
	Method               string  `json:"method"`
	Source               string  `json:"source"`
	MeshElements         int64   `json:"mesh_elements"`
	Above090             int64   `json:"above_0_90"`
	Above095             int64   `json:"above_0_95"`
	P95Lower             float64 `json:"p95_lower"`
	P95Upper             float64 `json:"p95_upper"`
	P95Precision         float64 `json:"p95_precision"`
	P95Definition        string  `json:"p95_definition"`
	ThresholdCountsExact bool    `json:"threshold_counts_exact"`
}

type OrthogonalQualityBound struct {
	Status       string  `json:"status"`
	Method       string  `json:"method"`
	Source       string  `json:"source"`
	MeshElements int64   `json:"mesh_elements"`
	MinLower     float64 `json:"min_lower"`
	MinUpper     float64 `json:"min_upper"`
	Precision    float64 `json:"precision"`
}

func validateSkewThresholdStats(s SkewThresholdStats, meshElements int64, nativeSkewMax *float64) error {
	if s.Status != "bounded" || s.Method != "native_worksheet_threshold" || !s.ThresholdCountsExact ||
		strings.TrimSpace(s.Source) == "" || s.P95Definition != "nearest_rank" ||
		meshElements <= 0 || s.MeshElements != meshElements ||
		s.Above090 < 0 || s.Above095 < 0 || s.Above095 > s.Above090 || s.Above090 > meshElements ||
		math.IsNaN(s.P95Lower) || math.IsNaN(s.P95Upper) ||
		s.P95Lower < 0 || s.P95Upper > 1 || s.P95Upper < s.P95Lower ||
		s.P95Precision <= 0 || s.P95Precision > 0.0011 ||
		math.Abs((s.P95Upper-s.P95Lower)-s.P95Precision) > 0.000000001 {
		return errors.New("invalid or incomplete native Worksheet threshold statistics")
	}
	if nativeSkewMax != nil {
		if *nativeSkewMax <= .90 && s.Above090 > 0 || *nativeSkewMax <= .95 && s.Above095 > 0 || s.P95Lower > *nativeSkewMax {
			return errors.New("Worksheet statistics contradict native maximum skewness")
		}
	}
	return nil
}

func validateOrthogonalQualityBound(b OrthogonalQualityBound, meshElements int64) error {
	if b.Status != "bounded" || b.Method != "native_worksheet_threshold" || strings.TrimSpace(b.Source) == "" ||
		meshElements <= 0 || b.MeshElements != meshElements ||
		math.IsNaN(b.MinLower) || math.IsNaN(b.MinUpper) ||
		b.MinLower < 0 || b.MinUpper > 1 || b.MinUpper < b.MinLower ||
		b.Precision <= 0 || b.Precision > 0.0011 ||
		math.Abs((b.MinUpper-b.MinLower)-b.Precision) > 0.000000001 {
		return errors.New("invalid native Worksheet orthogonal quality bounds")
	}
	return nil
}

// Boundaries deliberately split 0.90–0.95 and 0.95–1.00.
// Threshold counts are computed from exact values, NOT inferred from bins.
var skewBinEdges = []float64{0, .1, .2, .3, .4, .5, .6, .7, .8, .9, .95, 1}

func exactP95(sorted []float64) float64 {
	return sorted[int(math.Ceil(.95*float64(len(sorted))))-1]
}

// AggregateNativeSkewness computes exact counts/nearest-rank P95 with bounded
// O(N) memory. No synthetic defaults or inferred missing elements are allowed.
// Source authentication and obtaining native samples belong to the ACT adapter,
// which must explicitly verify mesh identity before supplying the records.
func AggregateNativeSkewness(samples []NativeSkewSample, meshElements int64, nativeSource string) (SkewnessDistribution, error) {
	if meshElements <= 0 || int64(len(samples)) != meshElements {
		return SkewnessDistribution{}, fmt.Errorf("native skewness coverage incomplete: got %d, mesh has %d", len(samples), meshElements)
	}
	if strings.TrimSpace(nativeSource) == "" {
		return SkewnessDistribution{}, errors.New("native skewness provenance missing")
	}
	out := SkewnessDistribution{Status: "complete", Method: "native_per_element", Source: nativeSource, SampleCount: meshElements}
	values := make([]float64, 0, len(samples))
	out.Bins = make([]SkewHistogramBin, len(skewBinEdges)-1)
	for i := range out.Bins {
		out.Bins[i] = SkewHistogramBin{Lower: skewBinEdges[i], Upper: skewBinEdges[i+1]}
	}
	seen := make(map[int64]struct{}, len(samples))
	perBody := make(map[int64][]float64)
	bodySums := make(map[int64]float64)
	bodyHigh90 := make(map[int64]int64)
	bodyHigh95 := make(map[int64]int64)
	bodyMax := make(map[int64]float64)
	completeBodyIdentity := true
	for _, sample := range samples {
		v := sample.Skewness
		if sample.ElementID <= 0 || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return SkewnessDistribution{}, fmt.Errorf("invalid native skewness value or element ID: %d", sample.ElementID)
		}
		if _, exists := seen[sample.ElementID]; exists {
			return SkewnessDistribution{}, fmt.Errorf("duplicate native element ID %d", sample.ElementID)
		}
		seen[sample.ElementID] = struct{}{}
		values = append(values, v)
		if v > .90 {
			out.Above090++
		}
		if v > .95 {
			out.Above095++
		}
		index := sort.Search(len(skewBinEdges)-1, func(i int) bool { return v <= skewBinEdges[i+1] })
		if index >= len(out.Bins) {
			return SkewnessDistribution{}, errors.New("skewness out of histogram range")
		}
		out.Bins[index].Count++
		if sample.BodyID <= 0 {
			completeBodyIdentity = false
		} else {
			perBody[sample.BodyID] = append(perBody[sample.BodyID], v)
			bodySums[sample.BodyID] += v
			if v > bodyMax[sample.BodyID] {
				bodyMax[sample.BodyID] = v
			}
			if v > .90 {
				bodyHigh90[sample.BodyID]++
			}
			if v > .95 {
				bodyHigh95[sample.BodyID]++
			}
		}
	}
	sort.Float64s(values)
	out.P95 = exactP95(values)
	// Region statistics are emitted only when every native element has a
	// unique, resolved Body identity. Partial regions are never presented as
	// a complete global Body comparison.
	if completeBodyIdentity {
		bodyIDs := make([]int64, 0, len(perBody))
		for id := range perBody {
			bodyIDs = append(bodyIDs, id)
		}
		sort.Slice(bodyIDs, func(i, j int) bool { return bodyIDs[i] < bodyIDs[j] })
		for _, id := range bodyIDs {
			vals := perBody[id]
			sort.Float64s(vals)
			out.BodyStats = append(out.BodyStats, BodySkewStatistics{
				BodyID: id, Count: int64(len(vals)), Mean: bodySums[id] / float64(len(vals)),
				Max: bodyMax[id], P95: exactP95(vals),
				Above090: bodyHigh90[id], Above095: bodyHigh95[id],
			})
		}
	}
	return out, nil
}

// validateSkewnessDistribution refuses unproven or partial aggregate records.
// Caller may render unavailable rather than silently substituting zeros.
func validateSkewnessDistribution(d SkewnessDistribution, meshElements int64) error {
	if d.Status != "complete" || d.Method != "native_per_element" || strings.TrimSpace(d.Source) == "" {
		return errors.New("no verified native per-element skewness distribution")
	}
	if meshElements <= 0 || d.SampleCount != meshElements || len(d.Bins) != len(skewBinEdges)-1 {
		return errors.New("native distribution has incomplete mesh coverage or bins")
	}
	if math.IsNaN(d.P95) || math.IsInf(d.P95, 0) || d.P95 < 0 || d.P95 > 1 ||
		d.Above090 < 0 || d.Above095 < 0 || d.Above095 > d.Above090 || d.Above090 > meshElements {
		return errors.New("native distribution has invalid metrics")
	}
	var total int64
	for i, bin := range d.Bins {
		if bin.Lower != skewBinEdges[i] || bin.Upper != skewBinEdges[i+1] || bin.Count < 0 {
			return errors.New("native distribution has invalid histogram bins")
		}
		total += bin.Count
	}
	if total != meshElements {
		return errors.New("native histogram does not cover mesh")
	}
	// The last two bins are (0.90,0.95] and (0.95,1.00], so
	// these strict high-skew counts must agree exactly with the histogram.
	if d.Above090 != d.Bins[len(d.Bins)-2].Count+d.Bins[len(d.Bins)-1].Count ||
		d.Above095 != d.Bins[len(d.Bins)-1].Count {
		return errors.New("native high-skew counts disagree with histogram")
	}
	var rank, running int64
	rank = int64(math.Ceil(.95 * float64(meshElements)))
	for i, bin := range d.Bins {
		running += bin.Count
		if running >= rank {
			if d.P95 > bin.Upper || (d.P95 <= bin.Lower && i != 0) || (i == 0 && d.P95 < bin.Lower) {
				return errors.New("native P95 disagrees with histogram interval")
			}
			break
		}
	}
	var bodyTotal int64
	seenBody := make(map[int64]bool)
	for _, b := range d.BodyStats {
		if b.BodyID <= 0 || seenBody[b.BodyID] || b.Count <= 0 ||
			math.IsNaN(b.Mean) || math.IsNaN(b.Max) || math.IsNaN(b.P95) ||
			b.Mean < 0 || b.Mean > 1 || b.Max < 0 || b.Max > 1 || b.P95 < 0 || b.P95 > 1 ||
			b.Above090 < 0 || b.Above095 < 0 || b.Above095 > b.Above090 || b.Above090 > b.Count {
			return errors.New("invalid Body skewness distribution")
		}
		bodyTotal += b.Count
		seenBody[b.BodyID] = true
	}
	if len(d.BodyStats) > 0 && bodyTotal != meshElements {
		return errors.New("native Body statistics do not cover mesh")
	}
	if len(d.BodyStats) > 0 {
		var sum90, sum95 int64
		for _, b := range d.BodyStats {
			sum90 += b.Above090
			sum95 += b.Above095
		}
		if sum90 != d.Above090 || sum95 != d.Above095 {
			return errors.New("native Body high-skew totals disagree with global counts")
		}
	}
	return nil
}

// comparableNativeBodyStats permits a Body-level before/after claim only when
// project, mesh system, geometry identity, quality definition and native metric
// source all match. A missing Body is NOT interpreted as zero bad cells.
func comparableNativeBodyStats(before, after QualitySnapshot, bodyID int64) (BodySkewStatistics, BodySkewStatistics, bool) {
	if ok, _ := qualitySnapshotsComparable(before, after); !ok ||
		before.SkewDistribution == nil || after.SkewDistribution == nil ||
		before.SkewDistribution.Source != after.SkewDistribution.Source || bodyID <= 0 {
		return BodySkewStatistics{}, BodySkewStatistics{}, false
	}
	var b, a BodySkewStatistics
	bFound, aFound := false, false
	for _, r := range before.SkewDistribution.BodyStats {
		if r.BodyID == bodyID {
			b, bFound = r, true
			break
		}
	}
	for _, r := range after.SkewDistribution.BodyStats {
		if r.BodyID == bodyID {
			a, aFound = r, true
			break
		}
	}
	return b, a, bFound && aFound
}
