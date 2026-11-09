//go:build windows

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *state) loadQualityHistory() error {
	history, err := a.ws.LoadQualityHistory()
	if err != nil {
		a.analysisError = err.Error()
		return err
	}
	a.qualityHistory = history
	a.analysisError = ""
	for _, key := range []string{"analysisbase", "analysistarget"} {
		send(a.ctl(key), CB_RESETCONTENT, 0, 0)
	}
	for _, item := range history {
		meshSignature := "指纹未取得"
		if item.MeshSignature != "" {
			meshSignature = "网格指纹 " + shorten(item.MeshSignature, 8)
		}
		label := fmt.Sprintf("%s · %s · %s · %s 单元 · %s", item.CompletedAt.Format("2006-01-02 15:04"), shorten(item.MeshSystem, 10), taskDisplayName(item.Phase), formatCount(item.MeshElements), meshSignature)
		add(a.ctl("analysisbase"), label)
		add(a.ctl("analysistarget"), label)
	}
	if len(history) > 0 {
		a.analysisBaseIndex = maxInt(0, len(history)-2)
		a.analysisTargetIndex = len(history) - 1
		send(a.ctl("analysisbase"), CB_SETCURSEL, uintptr(a.analysisBaseIndex), 0)
		send(a.ctl("analysistarget"), CB_SETCURSEL, uintptr(a.analysisTargetIndex), 0)
	} else {
		a.analysisBaseIndex, a.analysisTargetIndex = -1, -1
	}
	a.refreshAnalysisRegionList()
	refresh()
	return nil
}

func formatCount(n int64) string {
	if n <= 0 {
		return "未取得"
	}
	return strconv.FormatInt(n, 10)
}

func metricText(v *float64) string {
	if v == nil {
		return "未取得"
	}
	return fmt.Sprintf("%.4f", *v)
}

func (a *state) analysisSelection() (QualitySnapshot, QualitySnapshot, bool) {
	if len(a.qualityHistory) == 0 {
		return QualitySnapshot{}, QualitySnapshot{}, false
	}
	beforeIndex := int(send(a.ctl("analysisbase"), CB_GETCURSEL, 0, 0))
	afterIndex := int(send(a.ctl("analysistarget"), CB_GETCURSEL, 0, 0))
	if beforeIndex < 0 || beforeIndex >= len(a.qualityHistory) || afterIndex < 0 || afterIndex >= len(a.qualityHistory) {
		return QualitySnapshot{}, QualitySnapshot{}, false
	}
	return a.qualityHistory[beforeIndex], a.qualityHistory[afterIndex], true
}

func (a *state) drawAnalysis(dc uintptr, x, cw int) {
	textAt(dc, "网格质量分析中心", x+16, 83, cw-32, 40, 25, 720, navy)
	textAt(dc, "基于每个任务独立回执建立历史；不重新扫描 ANSYS，不把抽样值当作全网格统计。", x+16, 122, cw-32, 24, 13, 450, slate)
	textAt(dc, "基准版本", x+30, 183, 100, 20, 12, 550, slate)
	textAt(dc, "目标版本", x+295, 183, 100, 20, 12, 550, slate)
	if a.analysisError != "" {
		textAt(dc, "历史读取失败："+shorten(a.analysisError, 100), x+30, 250, cw-60, 30, 13, 600, warningcolor)
		return
	}
	if len(a.qualityHistory) == 0 {
		solid(dc, x+30, 260, cw-60, 190, white, 14)
		textAt(dc, "尚无可用质量快照", x+55, 295, cw-110, 32, 19, 650, navy)
		textAt(dc, "先在顶部点击「更新质量」。读取结果将按任务 ID 保留，后续可选择任意两轮对比。", x+55, 340, cw-110, 48, 14, 450, slate)
		return
	}
	before, after, selected := a.analysisSelection()
	switch a.analysisTab {
	case 0:
		a.drawAnalysisOverview(dc, x, cw)
	case 1:
		if !selected {
			textAt(dc, "请选择基准版本和目标版本。", x+30, 270, cw-60, 28, 14, 500, slate)
			return
		}
		a.drawAnalysisComparison(dc, x, cw, before, after)
	case 2:
		a.drawAnalysisDistribution(dc, x, cw, after)
	case 3:
		a.drawAnalysisHistory(dc, x, cw)
	}
}

func (a *state) drawAnalysisOverview(dc uintptr, x, cw int) {
	latest := a.qualityHistory[len(a.qualityHistory)-1]
	cardY, cardH, gap := 260, 112, 12
	cardW := (cw - 60 - 4*gap) / 5
	cards := []struct{ title, value, note string }{
		{"网格单元数", formatCount(latest.MeshElements), "ANSYS ACT 实测"},
		{"最大偏度", metricText(latest.SkewMax), "越低越好"},
		{"平均偏度", metricText(latest.SkewMean), "越低越好"},
		{"最小单元质量", metricText(latest.EQMin), "Element Quality · 越高越好"},
		{"平均单元质量", metricText(latest.EQMean), "Element Quality · 越高越好"},
	}
	for i, item := range cards {
		cx := x + 30 + i*(cardW+gap)
		solid(dc, cx, cardY, cardW, cardH, white, 12)
		textAt(dc, item.title, cx+14, cardY+13, cardW-28, 22, 13, 550, slate)
		textAt(dc, item.value, cx+14, cardY+39, cardW-28, 32, 21, 700, navy)
		textAt(dc, item.note, cx+14, cardY+79, cardW-28, 18, 10, 400, slate)
	}
	points := CompatibleQualityTrend(a.qualityHistory, latest)
	solid(dc, x+30, 388, cw-60, 243, white, 12)
	textAt(dc, "同工程质量趋势 · 最大偏度、平均偏度、平均单元质量（缺失指标留空）", x+50, 402, cw-100, 24, 14, 650, navy)
	a.drawMultiMetricTrend(dc, x+64, 444, cw-128, 154, points, false)
	solid(dc, x+30, 647, cw-60, 244, white, 12)
	textAt(dc, "高偏度单元比例趋势 · 严格 >0.90（蓝） · >0.95（橙）", x+50, 661, cw-100, 25, 14, 650, navy)
	a.drawMultiMetricTrend(dc, x+64, 704, cw-128, 156, points, true)
	textAt(dc, fmt.Sprintf("历史 %d 条 · 可比 %d 条 · 最近检测 %s", len(a.qualityHistory), len(points), latest.CompletedAt.Format("2006-01-02 15:04:05")), x+48, 892, cw-96, 18, 11, 500, slate)
	geometryState := "几何身份未核验"
	if latest.GeometryVerified {
		geometryState = "几何身份已核验"
	}
	meshSignature := "未取得"
	if latest.MeshSignature != "" {
		meshSignature = shorten(latest.MeshSignature, 12)
	}
	metrics := "原生逐单元分布未取得"
	if latest.SkewDistribution != nil {
		metrics = fmt.Sprintf("原生 P95 %.4f · >0.90 %d · >0.95 %d · Body %d", latest.SkewDistribution.P95, latest.SkewDistribution.Above090, latest.SkewDistribution.Above095, len(latest.SkewDistribution.BodyStats))
	} else if latest.SkewThresholds != nil {
		v := latest.SkewThresholds
		metrics = fmt.Sprintf("Worksheet P95 范围 %.4f～%.4f · >0.90 %d · >0.95 %d", v.P95Lower, v.P95Upper, v.Above090, v.Above095)
	}
	textAt(dc, "来源："+shorten(latest.Source, 25)+" · "+geometryState+" · 网格指纹 "+meshSignature+" · "+metrics, x+48, 911, cw-96, 18, 10, 450, slate)

}

// Two separately scaled [0,1] panels avoid mixing skewness/orthogonal metrics
// with high-skew cell fractions on a single misleading axis. Missing samples
// create gaps, never zero-valued points or interpolated histograms.
func (a *state) drawMultiMetricTrend(dc uintptr, x, y, w, h int, points []QualityTrendPoint, fractions bool) {
	left, right, top, bottom := x+42, x+w-18, y+8, y+h-27
	for i := 0; i <= 4; i++ {
		py := top + (bottom-top)*i/4
		rule(dc, left, py, right, py, lineclr)
		label := fmt.Sprintf("%.2f", 1-float64(i)*.25)
		if fractions {
			label = fmt.Sprintf("%.0f%%", 100-float64(i)*25)
		}
		textAt(dc, label, x, py-8, 38, 18, 10, 400, slate)
	}
	if len(points) < 2 {
		textAt(dc, "至少需要两条同工程、同几何、同来源和同质量定义的历史记录。", left+9, top+34, right-left-18, 36, 12, 450, slate)
		return
	}
	series := []struct {
		clr  uintptr
		read func(QualityTrendPoint) *float64
	}{}
	if fractions {
		series = []struct {
			clr  uintptr
			read func(QualityTrendPoint) *float64
		}{
			{cobalt, func(p QualityTrendPoint) *float64 { return p.High090Fraction }},
			{warningcolor, func(p QualityTrendPoint) *float64 { return p.High095Fraction }},
		}
	} else {
		series = []struct {
			clr  uintptr
			read func(QualityTrendPoint) *float64
		}{
			{cobalt, func(p QualityTrendPoint) *float64 { return p.SkewMaximum }},
			{green, func(p QualityTrendPoint) *float64 { return p.SkewAverage }},
			{warningcolor, func(p QualityTrendPoint) *float64 { return p.P95 }},
			{slate, func(p QualityTrendPoint) *float64 { return p.OrthogonalMinimum }},
			{color(123, 92, 185), func(p QualityTrendPoint) *float64 { return p.ElementQualityAverage }},
		}
	}
	usable := false
	for _, s := range series {
		havePrevious := false
		lastX, lastY := 0, 0
		for i, point := range points {
			v := s.read(point)
			if v == nil || *v < 0 || *v > 1 {
				havePrevious = false
				continue
			}
			usable = true
			px := left + (right-left)*i/(len(points)-1)
			py := bottom - int(*v*float64(bottom-top))
			if havePrevious {
				old := call(selectobj, dc, cachedPen(s.clr))
				call(moveto, dc, uintptr(lastX), uintptr(lastY), 0)
				call(lineto, dc, uintptr(px), uintptr(py))
				call(selectobj, dc, old)
			}
			solid(dc, px-3, py-3, 6, 6, s.clr, 3)
			havePrevious = true
			lastX, lastY = px, py
		}
	}
	if !usable {
		textAt(dc, "尚无可比的原生逐单元高偏度分布；此处不生成推算趋势。", left+9, top+34, right-left-18, 38, 12, 450, slate)
	}
	textAt(dc, points[0].Snapshot.CompletedAt.Format("01-02"), left, bottom+8, 60, 18, 10, 400, slate)
	textAt(dc, points[len(points)-1].Snapshot.CompletedAt.Format("01-02"), right-70, bottom+8, 70, 18, 10, 400, slate)
}

func (a *state) drawAnalysisComparison(dc uintptr, x, cw int, before, after QualitySnapshot) {
	ok, note := qualitySnapshotsComparable(before, after)
	referenceOK := false
	if !ok {
		var referenceNote string
		referenceOK, referenceNote = qualitySnapshotsReferenceComparable(before, after)
		if referenceOK {
			note = referenceNote
		}
	}
	if before.JobID == after.JobID {
		ok, referenceOK, note = false, false, "基准与目标是同一轮记录；不能声明优化变化"
	}
	statusColor := green
	if !ok {
		statusColor = warningcolor
	}
	// Unverified history can display labelled arithmetic deltas, not claims
	// about proven same-geometry optimization.
	numericOK := ok || referenceOK
	solid(dc, x+30, 258, cw-60, 58, white, 10)
	textAt(dc, note, x+48, 272, cw-96, 28, 13, 600, statusColor)
	oqDelta := "未取得"
	if before.OQMin != nil && after.OQMin != nil {
		if before.OQSource != "" && before.OQSource == after.OQSource {
			oqDelta = metricDelta(before.OQMin, after.OQMin)
		} else {
			oqDelta = "来源不一致/未记录"
		}
	}
	nativeComparable := numericOK && before.SkewDistribution != nil && after.SkewDistribution != nil &&
		before.SkewDistribution.Source == after.SkewDistribution.Source
	p95Delta, hi90Delta, hi95Delta := "未取得", "未取得", "未取得"
	if nativeComparable {
		p95Delta = fmt.Sprintf("%+.4f", after.SkewDistribution.P95-before.SkewDistribution.P95)
		hi90Delta = fmt.Sprintf("%+d", after.SkewDistribution.Above090-before.SkewDistribution.Above090)
		hi95Delta = fmt.Sprintf("%+d", after.SkewDistribution.Above095-before.SkewDistribution.Above095)
	}
	rows := []struct{ name, before, after, delta string }{
		{"总单元数", formatCount(before.MeshElements), formatCount(after.MeshElements), signedIntDelta(before.MeshElements, after.MeshElements)},
		{"节点数", formatCount(before.Nodes), formatCount(after.Nodes), signedIntDelta(before.Nodes, after.Nodes)},
		{"最大偏度", metricText(before.SkewMax), metricText(after.SkewMax), metricDelta(before.SkewMax, after.SkewMax)},
		{"平均偏度", metricText(before.SkewMean), metricText(after.SkewMean), metricDelta(before.SkewMean, after.SkewMean)},
		{"最小 Element Quality", metricText(before.EQMin), metricText(after.EQMin), "来源不一致/未取得"},
		{"平均单元质量（Element Quality）", metricText(before.EQMean), metricText(after.EQMean), "来源不一致/未取得"},
	}
	if numericOK && before.EQSource != "" && before.EQSource == after.EQSource {
		rows[4].delta = metricDelta(before.EQMin, after.EQMin)
		rows[5].delta = metricDelta(before.EQMean, after.EQMean)
	}
	// Unavailable OQ and full-distribution fields have no meaningful
	// comparison and must not occupy rows of repeated "未取得". Data is
	// not destroyed: as soon as either snapshot supplies real results,
	// the comparison includes the metric again.
	if before.OQMin != nil || after.OQMin != nil {
		rows = append(rows, struct{ name, before, after, delta string }{"最小正交质量", metricText(before.OQMin), metricText(after.OQMin), oqDelta})
	}
	thresholdComparable := numericOK && before.SkewThresholds != nil && after.SkewThresholds != nil &&
		before.SkewThresholds.Source == after.SkewThresholds.Source
	if thresholdComparable && !nativeComparable {
		hi90Delta = fmt.Sprintf("%+d", after.SkewThresholds.Above090-before.SkewThresholds.Above090)
		hi95Delta = fmt.Sprintf("%+d", after.SkewThresholds.Above095-before.SkewThresholds.Above095)
		p95Delta = "仅范围可比"
	}
	if before.SkewDistribution != nil || after.SkewDistribution != nil || before.SkewThresholds != nil || after.SkewThresholds != nil {
		rows = append(rows,
			struct{ name, before, after, delta string }{"P95 偏度", skewP95Text(before), skewP95Text(after), p95Delta},
			struct{ name, before, after, delta string }{"偏度 > 0.90 单元数", skewThresholdText(before, false), skewThresholdText(after, false), hi90Delta},
			struct{ name, before, after, delta string }{"偏度 > 0.95 单元数", skewThresholdText(before, true), skewThresholdText(after, true), hi95Delta})
		if !nativeComparable && !thresholdComparable {
			for i := len(rows) - 3; i < len(rows); i++ {
				rows[i].delta = "采集口径不同/不可比较"
			}
		}
	}
	if (before.OQBound != nil || after.OQBound != nil) && before.OQMin == nil && after.OQMin == nil {
		rows = append(rows, struct{ name, before, after, delta string }{"最小正交质量范围", orthogonalBoundText(before), orthogonalBoundText(after), "仅原生区间，无精确最小值"})
	}
	if !numericOK {
		for i := range rows {
			rows[i].delta = "不可比较"
		}
	}
	solid(dc, x+30, 332, cw-60, 430, white, 12)
	textAt(dc, "指标", x+52, 348, 170, 24, 12, 650, slate)
	textAt(dc, "优化前", x+270, 348, 150, 24, 12, 650, slate)
	textAt(dc, "优化后", x+465, 348, 150, 24, 12, 650, slate)
	changeTitle := "变化"
	if referenceOK {
		changeTitle = "参考差值（未核验）"
	}
	textAt(dc, changeTitle, x+660, 348, cw-710, 24, 12, 650, slate)
	rule(dc, x+48, 379, x+cw-48, 379, lineclr)
	for i, row := range rows {
		y := 391 + i*34
		textAt(dc, row.name, x+52, y, 190, 25, 14, 550, navy)
		textAt(dc, row.before, x+270, y, 150, 25, 13, 450, slate)
		textAt(dc, row.after, x+465, y, 150, 25, 13, 600, navy)
		textAt(dc, row.delta, x+660, y, cw-710, 25, 12, 500, slate)
		rule(dc, x+48, y+30, x+cw-48, y+30, lineclr)
	}
}

func skewP95Text(q QualitySnapshot) string {
	if q.SkewDistribution != nil {
		return fmt.Sprintf("%.4f", q.SkewDistribution.P95)
	}
	if q.SkewThresholds != nil {
		return fmt.Sprintf("%.4f～%.4f（范围）", q.SkewThresholds.P95Lower, q.SkewThresholds.P95Upper)
	}
	return "未取得"
}

func skewThresholdText(q QualitySnapshot, over095 bool) string {
	if q.SkewDistribution != nil {
		if over095 {
			return strconv.FormatInt(q.SkewDistribution.Above095, 10)
		}
		return strconv.FormatInt(q.SkewDistribution.Above090, 10)
	}
	if q.SkewThresholds != nil {
		if over095 {
			return strconv.FormatInt(q.SkewThresholds.Above095, 10)
		}
		return strconv.FormatInt(q.SkewThresholds.Above090, 10)
	}
	return "未取得"
}

func orthogonalBoundText(q QualitySnapshot) string {
	if q.OQBound == nil {
		return "未取得"
	}
	return fmt.Sprintf("%.4f～%.4f（范围）", q.OQBound.MinLower, q.OQBound.MinUpper)
}

func signedIntDelta(before, after int64) string {
	if before == 0 || after == 0 {
		return "未取得"
	}
	d := after - before
	return fmt.Sprintf("%+d", d)
}

func metricDelta(before, after *float64) string {
	if before == nil || after == nil {
		return "未取得"
	}
	return fmt.Sprintf("%+.4f", *after-*before)
}

func (a *state) drawAnalysisDistribution(dc uintptr, x, cw int, q QualitySnapshot) {
	if q.SkewDistribution != nil {
		d := q.SkewDistribution
		solid(dc, x+30, 258, cw-60, 385, white, 12)
		textAt(dc, "全网格原生偏度分布 · 精确计数", x+50, 274, cw-100, 28, 17, 650, navy)
		textAt(dc, fmt.Sprintf("P95 %.4f  ·  >0.90：%d  ·  >0.95：%d  ·  已覆盖 %d 个单元", d.P95, d.Above090, d.Above095, d.SampleCount), x+50, 309, cw-100, 26, 13, 550, slate)
		left, top, width, height := x+68, 366, cw-135, 208
		var maximum int64
		for _, bin := range d.Bins {
			if bin.Count > maximum {
				maximum = bin.Count
			}
		}
		if maximum > 0 {
			barW := maxInt(12, width/len(d.Bins)-12)
			for i, bin := range d.Bins {
				barH := int(float64(bin.Count) / float64(maximum) * float64(height-26))
				bx := left + i*width/len(d.Bins)
				if bin.Count > 0 && barH < 1 {
					barH = 1
				}
				barColor := cobalt
				if bin.Lower >= .90 {
					barColor = warningcolor
				}
				solid(dc, bx, top+height-barH, barW, barH, barColor, 3)
				if i%2 == 0 || bin.Lower >= .90 {
					textAt(dc, fmt.Sprintf("%.2g", bin.Lower), bx-4, top+height+6, barW+20, 18, 10, 400, slate)
				}
			}
		}
		textAt(dc, "平均单元质量："+metricText(q.EQMean)+" · "+shorten(q.EQSource, 75), x+50, 588, cw-100, 18, 11, 450, slate)
		textAt(dc, "偏度分布来源："+shorten(d.Source, 90)+"；严格 >0.90 / >0.95，P95 使用最近秩定义。", x+50, 612, cw-100, 18, 11, 450, slate)
		return
	}
	solid(dc, x+30, 258, cw-60, 170, white, 12)
	textAt(dc, "全网格偏度分布暂不可用", x+52, 282, cw-104, 30, 19, 650, navy)
	textAt(dc, "当前任务回执没有通过完整校验的原生逐单元偏度；普通质量检测仅采集全局指标。", x+52, 328, cw-104, 24, 13, 450, slate)
	textAt(dc, "可人工点击「采集原生分布」；若当前 ACT 接口不可用，仍显示未取得，不生成估算指标。", x+52, 358, cw-104, 24, 13, 600, warningcolor)
	textAt(dc, "当前平均单元质量："+metricText(q.EQMean)+"（独立 ACT 指标）", x+52, 391, cw-104, 22, 12, 500, navy)
	solid(dc, x+30, 446, cw-60, 190, white, 12)
	textAt(dc, "当前可用的官方 Worksheet 计数", x+52, 462, cw-104, 25, 16, 650, navy)
	if len(q.Worksheet) == 0 {
		textAt(dc, "本次任务没有返回可用 Worksheet 计数。", x+52, 510, cw-104, 24, 13, 450, slate)
		return
	}
	for i, row := range q.Worksheet {
		status := "接口不可用：" + row.Unavailable
		if row.Available {
			status = fmt.Sprintf("失败 %d · 警告 %d（沿用 ANSYS 当前 Worksheet 阈值）", row.Failed, row.Warning)
		}
		textAt(dc, row.Name, x+52, 500+i*34, 180, 24, 13, 550, navy)
		textAt(dc, status, x+240, 500+i*34, cw-300, 24, 12, 450, slate)
	}
}

func (a *state) drawAnalysisHistory(dc uintptr, x, cw int) {
	solid(dc, x+30, 258, cw-60, 360, white, 12)
	textAt(dc, "质量历史记录", x+52, 276, cw-104, 25, 17, 650, navy)
	textAt(dc, "时间", x+52, 318, 180, 22, 12, 650, slate)
	textAt(dc, "任务", x+250, 318, 130, 22, 12, 650, slate)
	textAt(dc, "单元数", x+410, 318, 120, 22, 12, 650, slate)
	textAt(dc, "最大偏度", x+550, 318, 120, 22, 12, 650, slate)
	textAt(dc, "正交最小 / 单元质量平均", x+690, 318, cw-740, 22, 12, 650, slate)
	rule(dc, x+50, 346, x+cw-50, 346, lineclr)
	start := maxInt(0, len(a.qualityHistory)-8)
	for i := start; i < len(a.qualityHistory); i++ {
		q := a.qualityHistory[i]
		y := 358 + (i-start)*30
		textAt(dc, q.CompletedAt.Format("01-02 15:04"), x+52, y, 180, 22, 12, 450, slate)
		textAt(dc, shorten(q.MeshSystem, 8)+" / "+taskDisplayName(q.Phase), x+250, y, 140, 22, 11, 500, navy)
		textAt(dc, formatCount(q.MeshElements), x+410, y, 120, 22, 12, 450, slate)
		textAt(dc, metricText(q.SkewMax), x+550, y, 120, 22, 12, 500, navy)
		textAt(dc, metricText(q.OQMin)+" / "+metricText(q.EQMean), x+690, y, cw-740, 22, 12, 450, slate)
	}
	solid(dc, x+30, 625, cw-60, 285, white, 12)
	textAt(dc, "Body 区域统计与前后变化 · 全部有效 Body 可滚动查看", x+48, 638, cw-96, 25, 15, 650, navy)
	base, target, selected := a.analysisSelection()
	if !selected || target.SkewDistribution == nil || len(target.SkewDistribution.BodyStats) == 0 {
		textAt(dc, "未取得完整原生 Body 身份，未生成区域统计或估算数值。", x+48, 664, cw-96, 20, 12, 500, warningcolor)
	} else {
		rows := QualityRegionComparisons(base, target)
		textAt(dc, fmt.Sprintf("目标轮 %d 个 Body；按偏度 >0.95 数量排序；只有身份和原生来源一致才显示差值。", len(rows)), x+48, 664, cw-96, 18, 11, 450, slate)
	}

}

func (a *state) exportQualityHistoryCSV() (string, error) {
	if len(a.qualityHistory) == 0 {
		return "", fmt.Errorf("没有可导出的质量快照")
	}
	name := "quality_history_" + time.Now().Format("20060102_150405_000000000") + ".csv"
	path := a.ws.path(name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	w := csv.NewWriter(f)
	rows := buildQualityHistoryCSVRows(a.qualityHistory)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (a *state) syncTaskDrawerControls() {
	open := a.taskCenterOpen
	visible(a.ctl("taskdrawertext"), open)
	visible(a.ctl("taskdrawerclose"), open)
	visible(a.ctl("taskdrawerlog"), open)
	visible(a.ctl("taskstop"), open && (a.busy || a.hasPendingStopTarget()))
	if open {
		set(a.ctl("taskdrawertext"), a.taskCenterDisplay())
	}
}

func (a *state) toggleTaskCenter() {
	a.taskCenterOpen = !a.taskCenterOpen
	if a.taskCenterOpen {
		a.taskCenterStatus = a.ws.taskStatus()
		a.taskCenterStatusUpdatedAt = time.Now()
		set(a.ctl("taskcenter"), "关闭任务")
	} else {
		set(a.ctl("taskcenter"), "任务中心")
	}
	a.syncTaskDrawerControls()
	a.layout()
	refresh()
}

func (a *state) taskCenterDisplay() string {
	a.mu.Lock()
	busy, phase, progress, details, percent := a.busy, a.activePhase, a.progress, a.details, a.progressPercent
	errText := a.errorText
	var last *Result
	if a.last != nil {
		copyResult := *a.last
		last = &copyResult
	}
	a.mu.Unlock()
	lines := []string{"任务中心", "状态：" + map[bool]string{true: "任务执行中", false: "当前无运行中的界面任务"}[busy]}
	if phase != "" {
		lines = append(lines, "操作："+taskDisplayName(phase))
	}
	if progress != "" {
		lines = append(lines, "当前阶段："+progress)
	}
	if details != "" {
		lines = append(lines, "阶段说明："+details)
	}
	if busy && percent >= 0 {
		lines = append(lines, fmt.Sprintf("可计量阶段进度：%d%%（不代表 ANSYS 总体完成率）", percent))
	}
	if last != nil {
		lines = append(lines, "最近回执："+taskDisplayName(last.Phase)+" / "+taskStateText(last.Status)+" / "+shorten(last.JobID, 12))
	}
	if errText != "" {
		lines = append(lines, "错误："+errText)
	}
	lines = append(lines, "", "持久化任务状态：", a.taskCenterStatus,
		"", "安全说明：打开任务中心只读取本地任务回执，不重新投递命令。停止请求不会强制终止 Meshing；任务锁只在 ACT 明确确认终态后释放。")
	return strings.Join(lines, "\r\n")
}

func (a *state) refreshTaskCenterText() {
	if !a.taskCenterOpen {
		return
	}
	now := time.Now()
	if taskStatusRefreshDue(a.taskCenterStatusUpdatedAt, now) {
		a.taskCenterStatus = a.ws.taskStatus()
		a.taskCenterStatusUpdatedAt = now
	}
	a.taskCenterText = a.taskCenterDisplay()
	set(a.ctl("taskdrawertext"), a.taskCenterText)
}

func (a *state) refreshAnalysisHistory() {
	if err := a.loadQualityHistory(); err != nil {
		a.message("读取质量历史失败：" + shorten(err.Error(), 120))
		return
	}
	a.message(fmt.Sprintf("已从任务回执读取 %d 条质量快照；未访问 Workbench。", len(a.qualityHistory)))
}

func (a *state) openQualityHistoryCSV() {
	path, err := a.exportQualityHistoryCSV()
	if err != nil {
		alert("质量历史 CSV 导出失败：" + err.Error())
		return
	}
	path = filepath.Clean(path)
	openReportPath(a.window, path)
	a.message("质量历史已导出：" + filepath.Base(path))
}

// The large Body table is an independent native scrollable read-only EDIT.
// Rendering the canvas never enumerates tens of thousands of Body rows.
func (a *state) refreshAnalysisRegionList() {
	h := a.ctl("analysisbodylist")
	if h == 0 {
		return
	}
	base, target, selected := a.analysisSelection()
	if !selected || target.SkewDistribution == nil || len(target.SkewDistribution.BodyStats) == 0 {
		set(h, "原生逐单元偏度或 Body ID 未完全取得。\r\n不会推算区域质量；请在 Windows 实机验证原生 ACT 采集接口。")
		return
	}
	rows := QualityRegionComparisons(base, target)
	var b strings.Builder
	fmt.Fprintf(&b, "目标任务 %s · %d 个原生 Body · 来源：%s\r\n", shorten(target.JobID, 14), len(rows), target.SkewDistribution.Source)
	b.WriteString("Body ID       单元数    平均偏度   最大偏度      P95      >0.90    >0.95      ΔP95      Δ>0.95\r\n")
	limit := len(rows)
	if limit > 5000 {
		limit = 5000
	}
	for _, row := range rows[:limit] {
		p95Delta, highDelta := "不可比较", "不可比较"
		if row.Comparable {
			p95Delta = fmt.Sprintf("%+.4f", *row.DeltaP95)
			highDelta = fmt.Sprintf("%+d", *row.DeltaHigh095)
		}
		q := row.Current
		fmt.Fprintf(&b, "%-10d %9d    %.4f     %.4f       %.4f     %7d   %7d    %s    %s\r\n",
			q.BodyID, q.Count, q.Mean, q.Max, q.P95, q.Above090, q.Above095, p95Delta, highDelta)
	}
	if limit < len(rows) {
		fmt.Fprintf(&b, "\r\n界面为保持流畅仅显示前 %d 个 Body；完整 %d 个区域数据请导出报告。\r\n", limit, len(rows))
	}
	set(h, b.String())
}
func (a *state) updateAnalysisRegionVisibility() {
	visible(a.ctl("analysisbodylist"), a.page == 4 && a.analysisTab == 3)
	if a.page == 4 && a.analysisTab == 3 {
		a.refreshAnalysisRegionList()
	}
}
func (a *state) openQualityAnalysisReport() {
	if len(a.qualityHistory) == 0 {
		alert("当前没有可导出的质量快照")
		return
	}
	base, target, valid := a.analysisSelection()
	if !valid {
		alert("请先选择基准和目标质量快照")
		return
	}
	bi, ti := -1, -1
	for i, q := range a.qualityHistory {
		if q.JobID == base.JobID {
			bi = i
		}
		if q.JobID == target.JobID {
			ti = i
		}
	}
	if bi < 0 || ti < 0 {
		alert("快照选择失效，请刷新历史后重试")
		return
	}
	path, err := ExportQualityAnalysisReport(a.ws, a.qualityHistory, bi, ti)
	if err != nil {
		alert("质量分析报告导出失败：" + shorten(err.Error(), 150))
		return
	}
	openReportPath(a.window, path)
	a.message("只读 HTML 质量分析报告已导出：" + filepath.Base(path))
}
