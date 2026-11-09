package main

// Human-facing labels are separate from ACT command identifiers. Keep the
// identifiers unchanged for task dispatch, logging, and persisted state.
func taskDisplayName(phase string) string {
	switch phase {
	case "sync_status":
		return "同步工程信息"
	case "inspect":
		return "完整几何扫描"
	case "quality_check":
		return "检查网格质量"
	case "mark_low_element_quality":
		return "标记低 Element Quality 单元"
	case "mark_high_skew":
		return "标记偏度区间单元"
	case "mark_warning":
		return "标记默认警告单元"
	case "mark_failed":
		return "标记失败单元"
	case "warning_refine_preview":
		return "生成局部优化方案"
	case "warning_refine_publish":
		return "发布局部 Sizing"
	case "publish_existing_sizing", "prepare":
		return "发布命名 Sizing"
	case "generate_reviewed":
		return "生成网格"
	case "export":
		return "导出网格"
	case "diagnose":
		return "风险诊断"
	case "rename_auxiliary", "publish_names":
		return "处理命名选择"
	case "":
		return "暂无任务"
	default:
		return phase
	}
}
func taskStateText(status string) string {
	switch status {
	case "ok", "success":
		return "已完成"
	case "error", "failed":
		return "失败"
	case "cancelled", "canceled":
		return "已取消"
	case "running":
		return "执行中"
	case "pending", "unknown":
		return "等待核实"
	case "":
		return "未获取状态"
	default:
		return status
	}
}
func qualityGateText(gate string) string {
	switch gate {
	case "critical_skewness":
		return "存在严重高偏度，需优化"
	case "review_required":
		return "需进一步审核"
	case "pass", "ok":
		return "指标达到当前检查要求"
	case "—":
		return "尚无质量结论"
	default:
		return gate
	}
}
