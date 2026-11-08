package main

import (
	"fmt"
	"strings"
)

func actualField(mesh map[string]any, key string) string {
	if mesh == nil {
		return "未获取"
	}
	raw, ok := mesh[key]
	if !ok || raw == nil {
		return "未获取"
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" || strings.EqualFold(text, "none") {
		return "未获取"
	}
	if (key == "ElementSize" || key == "MaximumSize") && (text == "0" || text == "0.0" || text == "0 [mm]" || text == "0.0 [mm]") {
		return "自动/未解析（不能视为 0 mm）"
	}
	return text
}
func globalReviewLines(snap *Snapshot, reference float64, proposedOverride bool) []string {
	if snap == nil {
		return []string{"当前没有 Meshing 扫描结果，先执行『只读扫描』。"}
	}
	src := "来源：上一次只读扫描（非实时写入值；生成时 ACT 将重新核验）"
	mesh := snap.Mesh
	rows := []struct{ key, label string }{
		{"ElementSize", "全局 Element Size"}, {"MaximumSize", "最大尺寸"}, {"GrowthRate", "增长率"},
		{"UseAdaptiveSizing", "自适应尺寸"}, {"CaptureCurvature", "曲率捕捉"},
		{"CaptureProximity", "邻近捕捉"}, {"PhysicsPreference", "物理偏好"},
		{"SolverPreference", "求解器偏好"}, {"ElementOrder", "单元阶次"},
	}
	lines := []string{src, "工程：" + trimRunes(snap.Project, 120), "几何指纹：" + trimRunes(snap.Fingerprint, 18)}
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%s：%s", r.label, actualField(mesh, r.key)))
	}
	lines = append(lines, fmt.Sprintf("AutoMesh 参考尺寸：%g mm（不是当前实际设置）", reference))
	if proposedOverride {
		lines = append(lines, "全局覆盖选项：已勾选；仅审核通过且无禁止情况时才允许写入")
	} else {
		lines = append(lines, "全局覆盖选项：关闭；不修改 Meshing 全局尺寸")
	}
	lines = append(lines, "边界层：需检查 Mesh.Children 中已有 Inflation 控制；不得依据缺失字段推断已关闭")
	return lines
}
func globalReviewText(snap *Snapshot, reference float64, override bool) string {
	return strings.Join(globalReviewLines(snap, reference, override), "\n")
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 2 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
