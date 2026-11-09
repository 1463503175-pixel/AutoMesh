package main

import (
	"fmt"
	"math"
)

// Diagnostics are read-only. 120000 is an advisory threshold, not a
// measured mesh or memory limit. Live controls and overlap must still pass.
const reviewBudgetLimit = 120000

type budgetDiagnosticRow struct {
	ID           string
	Proxy        int
	Relation     string
	BudgetStatus string
	Advice       string
}
type budgetDiagnosticView struct {
	Known            bool
	Historical       int
	Remaining        int
	MinimumCandidate int
	Summary          string
	Rows             []budgetDiagnosticRow
}

func candidateControlRelation(c WarningHotspot, old warningControlSnapshot) string {
	if c.BodyID <= 0 || old.BodyID != c.BodyID || len(c.CentreMM) != 3 || len(old.CentreMM) != 3 ||
		c.RadiusMM <= 0 || old.RadiusMM <= 0 {
		return ""
	}
	d := candidateCentersDistance(c, WarningHotspot{CentreMM: old.CentreMM})
	if math.IsInf(d, 0) || math.IsNaN(d) {
		return ""
	}
	switch {
	case d < 0.25*math.Min(c.RadiusMM, old.RadiusMM):
		return "近重合·禁止重复发布"
	case d+c.RadiusMM <= old.RadiusMM:
		return "候选球完全位于历史球内·需检查已有尺寸"
	case d+old.RadiusMM <= c.RadiusMM:
		return "候选球包含历史球·需检查过度细化"
	case d < c.RadiusMM+old.RadiusMM:
		return "影响球部分相交·需人工核验"
	default:
		return ""
	}
}

func computeBudgetDiagnostic(plan WarningPlan, existing []warningControlSnapshot, known bool) budgetDiagnosticView {
	v := budgetDiagnosticView{Known: known, Summary: "只读资源诊断；空间相交并不表示可扣除预算或已经造成网格问题。"}
	if known {
		v.Historical = warningSnapshotBudget(existing)
		v.Remaining = reviewBudgetLimit - v.Historical
		if v.Remaining < 0 {
			v.Remaining = 0
		}
	} else {
		v.Summary = "历史控件清单不完整，预算不可核验，禁止据此允许发布。"
	}
	for i, c := range plan.Clusters {
		if c.Proxy > 0 && (v.MinimumCandidate == 0 || c.Proxy < v.MinimumCandidate) {
			v.MinimumCandidate = c.Proxy
		}
		row := budgetDiagnosticRow{ID: fmt.Sprintf("#%03d (%s)", i+1, c.ID), Proxy: c.Proxy, Relation: "未发现同 Body 历史影响球交集", BudgetStatus: "历史预算未核验", Advice: "检查局部小面、薄间隙、网格尺寸过渡；不能仅凭低质量判断必须细化。"}
		matched := 0
		hasNear, inside := false, false
		for _, old := range existing {
			relation := candidateControlRelation(c, old)
			if relation == "" {
				continue
			}
			matched++
			if relation == "近重合·禁止重复发布" {
				hasNear = true
			}
			if relation == "候选球完全位于历史球内·需检查已有尺寸" {
				inside = true
			}
			if matched == 1 {
				row.Relation = fmt.Sprintf("%s：%s (Body %d)", old.Name, relation, old.BodyID)
			}
		}
		if matched > 1 {
			row.Relation += fmt.Sprintf("；另有 %d 处交叠", matched-1)
		}
		if hasNear {
			row.Advice = "禁止重复发布；先核对历史 Sizing、实际网格过渡及是否需要几何处理。"
		} else if inside {
			row.Advice = "已有同 Body 球完全覆盖候选球；优先检查现有尺寸和网格过渡，勿盲目加密。"
		} else if matched > 0 {
			row.Advice = "与历史影响球相交；核查尺寸过渡、狭缝和局部网格方法，不能按球体积直接抵扣预算。"
		}
		if known {
			switch {
			case hasNear:
				row.BudgetStatus = "近重合禁止发布"
			case c.Proxy <= 0:
				row.BudgetStatus = "代理值无效"
			case v.Historical > reviewBudgetLimit || c.Proxy > reviewBudgetLimit-v.Historical:
				row.BudgetStatus = fmt.Sprintf("超建议预算：单个代理 %d，建议余量 %d（需额外人工确认）", c.Proxy, v.Remaining)
			default:
				row.BudgetStatus = "低于建议阈值（仍需完整审核）"
			}
		}
		v.Rows = append(v.Rows, row)
	}
	if known && v.Historical >= reviewBudgetLimit {
		v.Summary = "历史 Sizing 预算代理已超过建议预警值：新候选仍可在查看历史覆盖、单元数与内存后额外人工确认；几何身份和近重合等硬门槛不会放松。"
	} else if known && v.MinimumCandidate > v.Remaining {
		v.Summary = "任何新增候选都会超过预算预警值：可继续人工审核，但须额外确认资源风险；不代表实际新增单元数。"
	}
	return v
}
