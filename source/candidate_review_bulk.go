package main

import "math"

// A bulk action changes the visible checkboxes only. It is not an ACT task,
// not consent to publish and never substitutes for per-selection risk review.
type reviewSelectionGroup string

const (
	reviewGroupAll     reviewSelectionGroup = "all"
	reviewGroupSevere  reviewSelectionGroup = "skew_ge_095"
	reviewGroupWarning reviewSelectionGroup = "skew_090_095"
	reviewGroupOther   reviewSelectionGroup = "skew_other_unknown"
)

type reviewBulkResult struct {
	IDs         []string
	Budget      int
	Skipped     int
	BudgetStops int
}

func candidateInReviewGroup(c WarningHotspot, group reviewSelectionGroup) bool {
	return candidateInMetricReviewGroup(c, group, "Skewness")
}

func candidateInMetricReviewGroup(c WarningHotspot, group reviewSelectionGroup, metric string) bool {
	if metric == "ElementQuality" {
		if group == reviewGroupAll {
			return true
		}
		if !c.EQVerified || c.EQMin == nil || math.IsNaN(*c.EQMin) || math.IsInf(*c.EQMin, 0) || *c.EQMin < 0 || *c.EQMin > 1 {
			return group == reviewGroupOther
		}
		switch group {
		case reviewGroupSevere:
			return *c.EQMin < 0.05
		case reviewGroupWarning:
			return *c.EQMin >= 0.05 && *c.EQMin < 0.10
		case reviewGroupOther:
			return *c.EQMin >= 0.10
		}
		return false
	}
	if group == reviewGroupAll {
		return true
	}
	// Classification is based solely on verified, real skewness. Unknown
	// values are never assigned an invented severity.
	if !c.SkewVerified || c.SkewMax == nil || math.IsNaN(*c.SkewMax) || math.IsInf(*c.SkewMax, 0) || *c.SkewMax < 0 || *c.SkewMax > 1 {
		return group == reviewGroupOther
	}
	switch group {
	case reviewGroupSevere:
		return *c.SkewMax >= 0.95
	case reviewGroupWarning:
		return *c.SkewMax >= 0.90 && *c.SkewMax < 0.95
	case reviewGroupOther:
		return *c.SkewMax < 0.90
	}
	return false
}

func candidateNearHistorical(c WarningHotspot, controls []warningControlSnapshot) bool {
	for _, old := range controls {
		if c.BodyID != old.BodyID || len(old.CentreMM) != 3 {
			continue
		}
		other := WarningHotspot{BodyID: old.BodyID, CentreMM: old.CentreMM, RadiusMM: old.RadiusMM}
		if candidateCentersDistance(c, other) < 0.25*math.Min(c.RadiusMM, old.RadiusMM) {
			return true
		}
	}
	return false
}

// selectReviewBulk remains fail-closed for duplicate/near-duplicate spheres,
// but the cumulative proxy is advisory. Existing
// publication identity must be checked by the caller BEFORE invoking it.
func selectReviewBulk(plan WarningPlan, group reviewSelectionGroup, published map[string]bool, existing []warningControlSnapshot, existingBudget int) reviewBulkResult {
	out := reviewBulkResult{Budget: existingBudget}
	// Incremental spatial checks keep a 4096-candidate plan at O(n²), not
	// O(n³) from repeatedly re-evaluating all previously checked pairs.
	chosen := make([]WarningHotspot, 0, len(plan.Clusters))
	seen := make(map[string]bool, len(plan.Clusters))
	for _, c := range plan.Clusters {
		if !candidateInMetricReviewGroup(c, group, plan.QualityMetric) {
			continue
		}
		if published[c.ID] || seen[c.ID] || len(c.ID) != 20 || c.BodyID <= 0 || len(c.CentreMM) != 3 || len(c.ElementIDs) == 0 ||
			c.Proxy <= 0 || c.RadiusMM <= 0 || c.RadiusMM > 12 || c.SizeMM < 0.5 || c.SizeMM > 5 ||
			(c.WaferGap != "manual_review_required" && c.WaferGap != "verified") || candidateNearHistorical(c, existing) {
			out.Skipped++
			continue
		}
		// Published candidates from the current plan and new checkbox picks
		// both block near-duplicate spheres. Mere sphere intersection remains
		// subject to manual clearance review, as in the original workflow.
		near := false
		for _, prior := range plan.Clusters {
			if !published[prior.ID] || prior.BodyID != c.BodyID || len(prior.CentreMM) != 3 {
				continue
			}
			if candidateCentersDistance(c, prior) < 0.25*math.Min(c.RadiusMM, prior.RadiusMM) {
				near = true
				break
			}
		}
		if !near {
			for _, prior := range chosen {
				if prior.BodyID == c.BodyID && candidateCentersDistance(c, prior) < 0.25*math.Min(c.RadiusMM, prior.RadiusMM) {
					near = true
					break
				}
			}
		}
		if near {
			out.Skipped++
			continue
		}
		out.IDs = append(out.IDs, c.ID)
		chosen = append(chosen, c)
		seen[c.ID] = true
		out.Budget += c.Proxy
	}
	return out
}

type reviewPublishGate struct {
	Busy           bool
	HistoricalLock string
	Selected       int
	Budget         int
	NearDuplicate  bool
	OverlapError   bool
	DetailsOpened  bool
	RiskConfirmed  bool
}

func (g reviewPublishGate) reason() string {
	switch {
	case g.Busy:
		return "已有 ACT 任务运行，不能重复投递。"
	case g.HistoricalLock != "":
		return "历史 Sizing 身份或预算尚未安全核对：" + g.HistoricalLock
	case g.Selected == 0:
		return "还没有勾选可发布的候选区域；已发布的候选不能重复选中。"
	case g.NearDuplicate:
		return "所选候选与历史 Sizing 影响球近乎重合，请取消冲突项。"
	case g.OverlapError:
		return "所选候选之间存在近重合或其他影响域冲突，请取消冲突项。"
	case !g.DetailsOpened:
		return "请先点击「查看全部影响域」，核查球心、作用 Body、片间净空及原有 Sizing。"
	case !g.RiskConfirmed:
		return "请勾选「我已核查所选区域的几何位置、晶圆间隙和现有 Sizing」。"
	}
	return ""
}
