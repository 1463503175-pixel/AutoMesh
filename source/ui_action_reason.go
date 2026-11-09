package main

// DenialReason is a user-facing explanation of the same button policy used by
// the native Win32 EnableWindow calls. It is not a replacement for dispatcher
// checks; dispatch-time identity validation remains authoritative.
func (s workflowAvailability) DenialReason(action string) string {
	if s.Allow(action) {
		return "可操作"
	}
	if s.Busy {
		return "已有任务运行中；不得重复提交或修改当前配置"
	}
	if !s.Ready {
		return "初始化尚未完成"
	}
	if action == "link" || action == "auto" {
		return "连接当前会话前请确认初始化状态"
	}
	if !s.Connected {
		return "尚未连接现有 Workbench 工程"
	}
	if !s.Ack {
		return "请确认已经在 Workbench 手动打开目标 Meshing 编辑器"
	}
	if !s.Selected {
		return "请在连接工程页选择当前 Mesh 系统"
	}
	switch action {
	case "markhighskew", "markloweq":
		if !s.Scanned {
			return "须先完成当前 Mesh 的完整几何扫描"
		}
		if !s.SkewRangeValid {
			return "质量指标范围不合法，应满足 0 ≤ 下限 < 上限 ≤ 1"
		}
	case "warningplan":
		if !s.Scanned {
			return "须先完成当前 Mesh 的完整几何扫描"
		}
		if !s.HasMarker {
			return "须先标记当前 Mesh 的警告或高偏度单元"
		}
	case "warningpublish":
		if !s.Scanned {
			return "须先完成当前 Mesh 的完整几何扫描"
		}
		if !s.HasPreview {
			return "须先生成并审核当前 Mesh 的局部优化候选方案"
		}
	case "fineprepare", "finechinese", "finebulkapply", "finegroupsave":
		if !s.Scanned {
			return "须先完成当前 Mesh 的完整几何扫描"
		}
		if !s.HasNamed {
			return "当前没有已选中的有效 Named Selection"
		}
	case "finegenerate":
		if !s.Scanned {
			return "须先完成当前 Mesh 的完整几何扫描"
		}
		if !s.CanGenerate {
			return "须先验证当前 Mesh 的命名 Sizing/历史接管清单"
		}
	default:
		if !s.Scanned {
			return "须先完成当前 Mesh 的完整几何扫描"
		}
	}
	return "当前依赖条件不满足；请核对工程、Mesh 身份和任务回执"
}
