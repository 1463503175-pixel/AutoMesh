//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func qualitySnapshotLabel(s QualitySnapshot) string {
	stamp := s.CapturedAt.Format("01-02 15:04")
	elements := "单元数未返回"
	if s.MeshElements != nil {
		elements = fmt.Sprintf("%d 单元", *s.MeshElements)
	}
	maxSkew := "最大偏度未返回"
	if s.Skewness.Max != nil {
		maxSkew = fmt.Sprintf("最大偏度 %.4f", *s.Skewness.Max)
	}
	return shorten(fmt.Sprintf("%s · %s · %s", stamp, elements, maxSkew), 82)
}

func defaultQualityPair(items []QualitySnapshot) (before, after int) {
	if len(items) == 0 {
		return -1, -1
	}
	after = len(items) - 1
	before = after
	for i := after - 1; i >= 0; i-- {
		if qualitySnapshotsComparable(items[i], items[after]) {
			before = i
			break
		}
	}
	return
}

func (a *state) populateQualityHistoryControls() {
	if a.ctl("analysisBefore") == 0 || a.ctl("analysisAfter") == 0 {
		return
	}
	a.mu.Lock()
	items := append([]QualitySnapshot(nil), a.qualityHistory...)
	a.mu.Unlock()

	for _, name := range []string{"analysisBefore", "analysisAfter"} {
		h := a.ctl(name)
		send(h, CB_RESETCONTENT, 0, 0)
		for _, item := range items {
			add(h, qualitySnapshotLabel(item))
		}
	}
	before, after := defaultQualityPair(items)
	if before >= 0 {
		send(a.ctl("analysisBefore"), CB_SETCURSEL, uintptr(before), 0)
		send(a.ctl("analysisAfter"), CB_SETCURSEL, uintptr(after), 0)
	}
	a.updateActionStates()
}

func (a *state) recordQualitySnapshot(result Result, summary map[string]any) {
	a.mu.Lock()
	project, fingerprint := "", ""
	if a.snapshot != nil {
		project = a.snapshot.Project
		fingerprint = a.snapshot.Fingerprint
	}
	a.mu.Unlock()

	snapshot, ok := qualitySnapshotFromSummary(result, summary, project, fingerprint, time.Now())
	if !ok {
		return
	}
	items, err := a.ws.SaveQualitySnapshot(snapshot)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.errorText = "质量检测已完成，但历史快照保存失败：" + err.Error()
		return
	}
	a.qualityHistory = items
}

func (a *state) selectedQualitySnapshots() (QualitySnapshot, QualitySnapshot, bool, string) {
	if len(a.qualityHistory) == 0 {
		return QualitySnapshot{}, QualitySnapshot{}, false, "暂无历史质量快照"
	}
	beforeIndex := int(send(a.ctl("analysisBefore"), CB_GETCURSEL, 0, 0))
	afterIndex := int(send(a.ctl("analysisAfter"), CB_GETCURSEL, 0, 0))
	if beforeIndex < 0 || beforeIndex >= len(a.qualityHistory) || afterIndex < 0 || afterIndex >= len(a.qualityHistory) {
		return QualitySnapshot{}, QualitySnapshot{}, false, "请选择有效的历史快照"
	}
	before, after := a.qualityHistory[beforeIndex], a.qualityHistory[afterIndex]
	if beforeIndex == afterIndex {
		return before, after, false, "当前只选中一轮快照；至少选择两轮检测结果才能对比"
	}
	if !qualitySnapshotsComparable(before, after) {
		return before, after, false, "两个快照的工程或几何指纹不同，不能作为同一模型的优化前后对比"
	}
	return before, after, true, ""
}

func qualityFloatText(v *float64) string {
	if v == nil {
		return "未获取"
	}
	return fmt.Sprintf("%.4f", *v)
}

func qualityIntText(v *int64) string {
	if v == nil {
		return "未获取"
	}
	return fmt.Sprintf("%d", *v)
}

func qualityFloatDelta(before, after *float64, higherIsBetter bool) string {
	if before == nil || after == nil {
		return "不可比"
	}
	delta := *after - *before
	if delta > -0.00005 && delta < 0.00005 {
		return "无明显变化"
	}
	direction := "↑"
	improved := delta > 0
	if delta < 0 {
		direction = "↓"
	}
	if !higherIsBetter {
		improved = !improved
	}
	if improved {
		return "改善 " + direction + fmt.Sprintf("%.4f", absFloat(delta))
	}
	return "变差 " + direction + fmt.Sprintf("%.4f", absFloat(delta))
}

func qualityIntDelta(before, after *int64) string {
	if before == nil || after == nil {
		return "不可比"
	}
	delta := *after - *before
	if delta == 0 {
		return "无变化"
	}
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func worksheetByName(s QualitySnapshot, name string) (failed, warning *int64) {
	for _, item := range s.Worksheet {
		if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(name)) {
			return item.Failed, item.Warning
		}
	}
	return nil, nil
}

func (a *state) drawQualityAnalysis(dc uintptr, x, cw int) {
	textAt(dc, "网格质量分析", x+16, 83, cw-32, 43, 25, 700, navy)
	textAt(dc, "保存的质量快照 · 同一工程与几何版本的多轮对比 · 真实指标趋势", x+16, 130, cw-32, 25, 13, 400, slate)

	if len(a.qualityHistory) == 0 {
		solid(dc, x, 215, cw, 230, white, 18)
		textAt(dc, "暂无历史质量快照", x+30, 245, cw-60, 32, 19, 650, navy)
		textAt(dc, "连接现有 Workbench 后，先使用顶部「同步工程」，再点击「更新质量」。", x+30, 296, cw-60, 26, 14, 450, slate)
		textAt(dc, "每次成功的质量检测会自动保存一条独立快照；切换页面不会重复调用 ANSYS。", x+30, 333, cw-60, 26, 13, 450, slate)
		textAt(dc, "本页只展示 ACT 实际返回的数据，不把缺失指标替换为 0。", x+30, 371, cw-60, 26, 12, 500, warningcolor)
		return
	}

	before, after, comparable, reason := a.selectedQualitySnapshots()
	latest := a.qualityHistory[len(a.qualityHistory)-1]
	display := after
	if display.ID == "" {
		display = latest
	}
	cardY, cardH := 215, 92
	gap := 12
	cardW := (cw - 3*gap) / 4
	cards := []struct {
		title, value, note string
		color              uintptr
	}{
		{"最大偏斜度", qualityFloatText(display.Skewness.Max), "ACT 实测最大值", cobalt},
		{"平均偏斜度", qualityFloatText(display.Skewness.Mean), "ACT 实测平均值", navy},
		{"网格单元数", qualityIntText(display.MeshElements), "当前快照", navy},
		{"最小正交质量", "未获取", "ACT 未返回有效值", warningcolor},
	}
	if display.OrthogonalQuality != nil {
		cards[3].value = qualityFloatText(display.OrthogonalQuality.Min)
		cards[3].note = "ACT 原生指标"
		cards[3].color = green
	}
	for i, card := range cards {
		cx := x + i*(cardW+gap)
		solid(dc, cx, cardY, cardW, cardH, white, 14)
		textAt(dc, card.title, cx+15, cardY+10, cardW-30, 20, 12, 550, slate)
		textAt(dc, card.value, cx+15, cardY+33, cardW-30, 30, 19, 700, card.color)
		textAt(dc, card.note, cx+15, cardY+66, cardW-30, 17, 10, 400, slate)
	}

	// Comparison table: values are displayed only when both snapshots contain them.
	tableY := 320
	solid(dc, x, tableY, cw, 228, white, 16)
	textAt(dc, "关键指标对比", x+20, tableY+12, cw-40, 24, 16, 700, navy)
	textAt(dc, "指标", x+22, tableY+47, 170, 20, 11, 650, slate)
	beforeLabel, afterLabel := "优化前", "优化后"
	if !comparable {
		beforeLabel, afterLabel = "快照 A", "快照 B"
	}
	textAt(dc, beforeLabel, x+225, tableY+47, 150, 20, 11, 650, slate)
	textAt(dc, afterLabel, x+420, tableY+47, 150, 20, 11, 650, slate)
	textAt(dc, "变化", x+620, tableY+47, cw-650, 20, 11, 650, slate)
	rule(dc, x+20, tableY+70, x+cw-20, tableY+70, lineclr)

	rows := []struct {
		name   string
		before string
		after  string
		delta  string
	}{
		{"最大偏斜度", qualityFloatText(before.Skewness.Max), qualityFloatText(after.Skewness.Max), qualityFloatDelta(before.Skewness.Max, after.Skewness.Max, false)},
		{"平均偏斜度", qualityFloatText(before.Skewness.Mean), qualityFloatText(after.Skewness.Mean), qualityFloatDelta(before.Skewness.Mean, after.Skewness.Mean, false)},
		{"最小偏斜度", qualityFloatText(before.Skewness.Min), qualityFloatText(after.Skewness.Min), qualityFloatDelta(before.Skewness.Min, after.Skewness.Min, false)},
		{"最小正交质量", "未获取", "未获取", "不可比"},
		{"网格单元数", qualityIntText(before.MeshElements), qualityIntText(after.MeshElements), qualityIntDelta(before.MeshElements, after.MeshElements)},
		{"网格节点数", qualityIntText(before.MeshNodes), qualityIntText(after.MeshNodes), qualityIntDelta(before.MeshNodes, after.MeshNodes)},
	}
	if before.OrthogonalQuality != nil && after.OrthogonalQuality != nil {
		rows[3] = struct {
			name string
			before string
			after string
			delta string
		}{"最小正交质量", qualityFloatText(before.OrthogonalQuality.Min), qualityFloatText(after.OrthogonalQuality.Min), qualityFloatDelta(before.OrthogonalQuality.Min, after.OrthogonalQuality.Min, true)}
	}
	if !comparable {
		for i := range rows {
			rows[i].delta = "不可比"
		}
	}
	for i, row := range rows {
		y := tableY + 77 + i*23
		textAt(dc, row.name, x+22, y, 180, 20, 12, 500, navy)
		textAt(dc, row.before, x+225, y, 150, 20, 12, 450, slate)
		textAt(dc, row.after, x+420, y, 150, 20, 12, 600, navy)
		deltaColor := slate
		if strings.HasPrefix(row.delta, "改善") {
			deltaColor = green
		} else if strings.HasPrefix(row.delta, "变差") {
			deltaColor = warningcolor
		}
		textAt(dc, row.delta, x+620, y, cw-650, 20, 11, 550, deltaColor)
	}
	if !comparable {
		textAt(dc, reason, x+22, tableY+207, cw-44, 18, 10, 550, warningcolor)
	}

	// Trend chart uses only saved ACT min/max/mean aggregates. It is not a
	// per-element histogram and never implies a distribution that ACT did not return.
	chartY := 562
	solid(dc, x, chartY, cw, 220, white, 16)
	textAt(dc, "同一几何版本的质量趋势", x+20, chartY+12, cw-40, 24, 16, 700, navy)
	textAt(dc, "最大偏斜度", x+cw-230, chartY+16, 95, 18, 10, 550, cobalt)
	textAt(dc, "平均偏斜度", x+cw-125, chartY+16, 100, 18, 10, 550, green)
	trend := make([]QualitySnapshot, 0, 8)
	for _, item := range a.qualityHistory {
		if qualitySnapshotsComparable(item, display) && item.Skewness.Max != nil {
			trend = append(trend, item)
		}
	}
	if len(trend) > 8 {
		trend = trend[len(trend)-8:]
	}
	plotX, plotY := x+54, chartY+49
	plotW, plotH := cw-82, 126
	for tick := 0; tick <= 4; tick++ {
		y := plotY + plotH - tick*plotH/4
		rule(dc, plotX, y, plotX+plotW, y, lineclr)
		textAt(dc, fmt.Sprintf("%.2f", float64(tick)/4), x+12, y-8, 36, 16, 9, 400, slate)
	}
	if len(trend) == 0 {
		textAt(dc, "没有可用于趋势图的同几何偏斜度快照。", plotX+10, plotY+42, plotW-20, 22, 12, 450, slate)
	} else {
		var previousMaxX, previousMaxY, previousMeanX, previousMeanY int
		hasPreviousMean := false
		for i, item := range trend {
			px := plotX
			if len(trend) > 1 {
				px += i * plotW / (len(trend) - 1)
			}
			maxY := plotY + plotH - int(clampQuality01(*item.Skewness.Max)*float64(plotH))
			if i > 0 {
				rule(dc, previousMaxX, previousMaxY, px, maxY, cobalt)
			}
			solid(dc, px-3, maxY-3, 6, 6, cobalt, 3)
			previousMaxX, previousMaxY = px, maxY
			if item.Skewness.Mean != nil {
				meanY := plotY + plotH - int(clampQuality01(*item.Skewness.Mean)*float64(plotH))
				if hasPreviousMean {
					rule(dc, previousMeanX, previousMeanY, px, meanY, green)
				}
				solid(dc, px-3, meanY-3, 6, 6, green, 3)
				previousMeanX, previousMeanY = px, meanY
				hasPreviousMean = true
			}
			if len(trend) <= 8 {
				textAt(dc, item.CapturedAt.Format("01-02"), px-28, plotY+plotH+5, 56, 16, 9, 400, slate)
			}
		}
	}
	textAt(dc, "说明：当前 ACT 接口返回最小值、最大值、平均值及 Worksheet 计数；未返回逐单元完整分布，因此不绘制直方图。", x+20, chartY+193, cw-40, 18, 10, 450, slate)

	// Worksheet counts are kept by metric name; unlike metrics they are not
	// summed across rows, which would double-count the same element.
	worksheetY := 800
	solid(dc, x, worksheetY, cw, 154, white, 16)
	textAt(dc, "Worksheet 失败 / 警告计数", x+20, worksheetY+12, cw-40, 23, 15, 700, navy)
	textAt(dc, "质量项", x+22, worksheetY+43, 190, 18, 10, 600, slate)
	worksheetBeforeLabel, worksheetAfterLabel := "优化前 失败 / 警告", "优化后 失败 / 警告"
	if !comparable {
		worksheetBeforeLabel, worksheetAfterLabel = "快照 A 失败 / 警告", "快照 B 失败 / 警告"
	}
	textAt(dc, worksheetBeforeLabel, x+245, worksheetY+43, 200, 18, 10, 600, slate)
	textAt(dc, worksheetAfterLabel, x+490, worksheetY+43, cw-510, 18, 10, 600, slate)
	names := make([]string, 0, len(display.Worksheet))
	for _, metric := range display.Worksheet {
		names = append(names, metric.Name)
	}
	if len(names) == 0 {
		textAt(dc, "当前质量快照未返回可核验的 Worksheet 计数。", x+22, worksheetY+78, cw-44, 22, 12, 450, slate)
	} else {
		if len(names) > 4 {
			names = names[:4]
		}
		for i, name := range names {
			y := worksheetY + 68 + i*19
			bf, bw := worksheetByName(before, name)
			af, aw := worksheetByName(after, name)
			textAt(dc, shorten(name, 24), x+22, y, 190, 17, 11, 500, navy)
			textAt(dc, qualityIntText(bf)+" / "+qualityIntText(bw), x+245, y, 200, 17, 11, 450, slate)
			textAt(dc, qualityIntText(af)+" / "+qualityIntText(aw), x+490, y, cw-510, 17, 11, 550, navy)
		}
	}

	historyY := 970
	solid(dc, x, historyY, cw, 188, white, 16)
	textAt(dc, fmt.Sprintf("历史质量快照（保留最近 %d 轮）", maxQualityHistory), x+20, historyY+12, cw-40, 23, 15, 700, navy)
	textAt(dc, "检测时间", x+22, historyY+43, 180, 18, 10, 600, slate)
	textAt(dc, "网格单元数", x+240, historyY+43, 150, 18, 10, 600, slate)
	textAt(dc, "最大偏斜度", x+430, historyY+43, 150, 18, 10, 600, slate)
	textAt(dc, "工程 / 几何标识", x+620, historyY+43, cw-640, 18, 10, 600, slate)
	start := len(a.qualityHistory) - 6
	if start < 0 {
		start = 0
	}
	for i := start; i < len(a.qualityHistory); i++ {
		item := a.qualityHistory[i]
		y := historyY + 68 + (i-start)*18
		textAt(dc, item.CapturedAt.Format("2006-01-02 15:04"), x+22, y, 190, 16, 10, 450, slate)
		textAt(dc, qualityIntText(item.MeshElements), x+240, y, 150, 16, 10, 450, navy)
		textAt(dc, qualityFloatText(item.Skewness.Max), x+430, y, 150, 16, 10, 550, navy)
		textAt(dc, shorten(filepathBaseQuality(item.Project)+" · "+shorten(item.GeometryFingerprint, 10), 34), x+620, y, cw-640, 16, 9, 400, slate)
	}
}

func clampQuality01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func filepathBaseQuality(project string) string {
	project = strings.TrimRight(strings.ReplaceAll(project, "/", "\\"), "\\")
	if i := strings.LastIndex(project, "\\"); i >= 0 {
		return project[i+1:]
	}
	return project
}

func (a *state) exportQualityHistoryCSV() {
	a.mu.Lock()
	items := append([]QualitySnapshot(nil), a.qualityHistory...)
	a.mu.Unlock()
	if len(items) == 0 {
		alert("暂无可导出的质量快照。")
		return
	}
	path := a.ws.path("quality_history.csv")
	if err := a.ws.ExportQualityHistoryCSV(path, items); err != nil {
		alert("质量历史 CSV 导出失败：" + err.Error())
		return
	}
	a.message("质量历史已导出：" + path)
	if err := exec.Command("explorer.exe", "/select,", path).Start(); err != nil {
		// Export has succeeded; Explorer is only a convenience.
		a.message("CSV 已导出；打开文件位置失败：" + err.Error())
	}
}
