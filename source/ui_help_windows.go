//go:build windows

package main

import (
	"fmt"
	"strings"
)

func (a *state) showPageActionHelp() {
	view := a.sharedDataStatus()
	selection := int(send(a.ctl("systems"), CB_GETCURSEL, 0, 0))
	selected := selection >= 0 && selection < len(a.systems)
	ack := send(a.ctl("ack"), BM_GETCHECK, 0, 0) == 1
	scanned := view.GeometryState == SharedStateReady && selected
	marker := scanned && a.ws.hasCurrentMarker(a.snapshot)
	reviewed := false
	if scanned && !a.busy {
		_, err := a.ws.ReadReviewedWarningPlan(a.snapshot)
		reviewed = err == nil
	}
	available := workflowAvailability{
		Ready: a.ready, Busy: a.busy, Connected: view.ConnectionState == SharedStateReady && a.port > 0 && len(a.systems) > 0,
		Ack: ack, Selected: selected, Scanned: scanned, HasMarker: marker, HasPreview: reviewed,
		HasReport: a.ws.hasCurrentWarningReport(a.snapshot), HasNamed: scanned && len(a.selectedNamed) > 0,
		SkewRangeValid: validSkewRange(read(a.ctl("skewlow")), read(a.ctl("skewhigh"))),
		CanGenerate:    scanned && (a.preparedSystem == a.systems[selection] || hasVerifiedManagedManifest(a.ws.Dir, a.snapshot)),
	}
	markAction := markerRuleAction(int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0)))
	sections := map[int][]struct{ Key, Caption string }{
		0: {{"qualitycheck", "更新质量"}},
		1: {{"link", "连接已有工程"}, {"scan", "完整几何扫描"}},
		2: {{"scan", "完整几何扫描"}, {"fineprepare", "发布命名 Sizing"}, {"finegenerate", "生成网格（仅用户主动触发）"}},
		3: {{"qualitycheck", "质量检测"}, {markAction, "标记目标"}, {"warningplan", "生成局部方案"}, {"warningpublish", "审核发布 Sizing"}},
		4: {{"analysisdeep", "采集原生分布"}},
		5: {{"exportbutton", "网格导出"}},
		6: {{"warningpublish", "审核发布 Sizing"}},
	}
	lines := []string{"当前页面操作条件说明："}
	for _, item := range sections[a.page] {
		reason := available.DenialReason(item.Key)
		lines = append(lines, fmt.Sprintf("• %s：%s", item.Caption, reason))
	}
	if a.page == 4 {
		if len(a.qualityHistory) == 0 {
			lines = append(lines, "• 历史对比/报告导出：尚无本地质量回执")
		}
		lines = append(lines, "• 历史查看、对比与导出只读取本地回执，不调用 ANSYS。")
	}
	if a.page == 6 {
		lines = append(lines, "• 120000 为预算代理预警值，超限须额外人工确认；单批最多6个，且历史身份、重复球、净空审核仍为硬门槛。")
	}
	lines = append(lines, "", "安全限制：页面按钮只提示条件，最终由任务调度器复核工程指纹、真实回执和 Sizing 安全门槛。", "该页面不会因查看操作说明而启动任务。")
	notice(strings.Join(lines, "\n"))
}
