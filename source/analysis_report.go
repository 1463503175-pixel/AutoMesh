package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type reportMeasure struct{ Name, Before, After, Delta string }
type reportTrend struct {
	Time, JobID, MeshSignature, MeshElements, MaxSkew, MeanSkew, P95, OrthogonalMin, ElementQualityMin, ElementQualityMean, High090, High095 string
}
type reportRegion struct {
	ID, Count, Mean, Maximum, P95, High090, High095, DeltaP95, DeltaHigh095 string
}
type reportBin struct {
	Interval, Count, Portion string
	Width                    int
}
type qualityAnalysisReportData struct {
	Generated, Project, System, BaseJob, TargetJob, BaseTime, TargetTime        string
	Identity, GeometryFingerprint, BeforeMesh, AfterMesh, Definition            string
	QualitySource, SkewSource, OQSource, EQSource, NativeSource, NativeCoverage string
	Comparable, ReferenceComparison                                             bool
	Measures                                                                    []reportMeasure
	Trends                                                                      []reportTrend
	Regions                                                                     []reportRegion
	Histogram                                                                   []reportBin
	Missing                                                                     []string
	HistoryCount                                                                int
	ShowP95, ShowOQ, ShowHighSkew                                               bool
}

func reportFloat(p *float64) string {
	if p == nil || math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 0 || *p > 1 {
		return "未取得"
	}
	return fmt.Sprintf("%.5f", *p)
}
func reportFraction(p *float64) string {
	if p == nil {
		return "未取得"
	}
	return fmt.Sprintf("%.3f%%", *p*100)
}
func reportDelta(p *float64) string {
	if p == nil {
		return "不可比较/未取得"
	}
	return fmt.Sprintf("%+.5f", *p)
}
func reportInt(n int64, available bool) string {
	if !available {
		return "未取得"
	}
	return strconv.FormatInt(n, 10)
}

func reportWorksheetP95(q QualitySnapshot) string {
	if q.SkewThresholds == nil {
		return "未取得"
	}
	return fmt.Sprintf("%.5f～%.5f（范围）", q.SkewThresholds.P95Lower, q.SkewThresholds.P95Upper)
}
func reportWorksheetCount(q QualitySnapshot, over095 bool) string {
	if q.SkewThresholds == nil {
		return "未取得"
	}
	if over095 {
		return strconv.FormatInt(q.SkewThresholds.Above095, 10)
	}
	return strconv.FormatInt(q.SkewThresholds.Above090, 10)
}
func reportOQBound(q QualitySnapshot) string {
	if q.OQBound == nil {
		return "未取得"
	}
	return fmt.Sprintf("%.5f～%.5f（范围）", q.OQBound.MinLower, q.OQBound.MinUpper)
}

func BuildQualityAnalysisReport(history []QualitySnapshot, baseIndex, targetIndex int) (string, error) {
	if baseIndex < 0 || targetIndex < 0 || baseIndex >= len(history) || targetIndex >= len(history) {
		return "", fmt.Errorf("请选择有效的基准与目标质量快照")
	}
	before, after := history[baseIndex], history[targetIndex]
	comparable, note := qualitySnapshotsComparable(before, after)
	referenceOK := false
	if !comparable {
		var referenceNote string
		referenceOK, referenceNote = qualitySnapshotsReferenceComparable(before, after)
		if referenceOK {
			note = referenceNote
		}
	}
	if baseIndex == targetIndex {
		comparable, referenceOK = false, false
		note = "基准与目标为同一轮记录；不存在前后改善量"
	}
	numericOK := comparable || referenceOK
	nativeComparable := numericOK && before.SkewDistribution != nil && after.SkewDistribution != nil &&
		before.SkewDistribution.Source == after.SkewDistribution.Source
	oqComparable := numericOK && before.OQSource != "" && before.OQSource == after.OQSource
	eqComparable := numericOK && before.EQSource != "" && before.EQSource == after.EQSource
	data := qualityAnalysisReportData{
		Generated: time.Now().Format("2006-01-02 15:04:05"), Project: after.Project, System: after.MeshSystem,
		BaseJob: before.JobID, TargetJob: after.JobID, BaseTime: before.CompletedAt.Format(time.RFC3339), TargetTime: after.CompletedAt.Format(time.RFC3339),
		Identity: note, Comparable: comparable, ReferenceComparison: referenceOK, GeometryFingerprint: after.GeometryFingerprint, BeforeMesh: before.MeshSignature, AfterMesh: after.MeshSignature,
		Definition: after.QualityDefinition, QualitySource: after.Source, SkewSource: after.SkewSource, OQSource: after.OQSource, EQSource: after.EQSource,
		HistoryCount: len(history),
	}
	addFloat := func(name string, b, a *float64, comparable bool) {
		data.Measures = append(data.Measures, reportMeasure{Name: name, Before: reportFloat(b), After: reportFloat(a), Delta: reportDelta(metricDifference(b, a, comparable))})
	}
	data.Measures = append(data.Measures, reportMeasure{"网格单元数", reportInt(before.MeshElements, before.MeshElements > 0), reportInt(after.MeshElements, after.MeshElements > 0), "不可比较/未取得"})
	if numericOK && before.MeshElements > 0 && after.MeshElements > 0 {
		data.Measures[0].Delta = fmt.Sprintf("%+d", after.MeshElements-before.MeshElements)
	}
	addFloat("最大偏度", before.SkewMax, after.SkewMax, numericOK)
	addFloat("平均偏度", before.SkewMean, after.SkewMean, numericOK)
	if before.OQMin != nil || after.OQMin != nil {
		addFloat("最小正交质量", before.OQMin, after.OQMin, oqComparable)
	}
	addFloat("最小 Element Quality", before.EQMin, after.EQMin, eqComparable)
	addFloat("平均单元质量（Element Quality）", before.EQMean, after.EQMean, eqComparable)
	var p95b, p95a *float64
	if before.SkewDistribution != nil {
		p95b = &before.SkewDistribution.P95
	}
	if after.SkewDistribution != nil {
		p95a = &after.SkewDistribution.P95
	}
	if p95b != nil || p95a != nil {
		addFloat("P95 偏度（最近秩）", p95b, p95a, nativeComparable)
	} else if before.SkewThresholds != nil || after.SkewThresholds != nil {
		data.Measures = append(data.Measures, reportMeasure{"P95 偏度范围（Worksheet 阈值搜索）",
			reportWorksheetP95(before), reportWorksheetP95(after), "区间值，不能计算精确差"})
	}
	if p95b != nil || p95a != nil {
		for _, threshold := range []struct {
			name string
			get  func(*SkewnessDistribution) int64
		}{
			{"偏度严格 > 0.90", func(d *SkewnessDistribution) int64 { return d.Above090 }},
			{"偏度严格 > 0.95", func(d *SkewnessDistribution) int64 { return d.Above095 }},
		} {
			row := reportMeasure{Name: threshold.name, Before: "未取得", After: "未取得", Delta: "不可比较/未取得"}
			if before.SkewDistribution != nil {
				row.Before = strconv.FormatInt(threshold.get(before.SkewDistribution), 10)
			}
			if after.SkewDistribution != nil {
				row.After = strconv.FormatInt(threshold.get(after.SkewDistribution), 10)
			}
			if nativeComparable {
				row.Delta = fmt.Sprintf("%+d", threshold.get(after.SkewDistribution)-threshold.get(before.SkewDistribution))
			}
			data.Measures = append(data.Measures, row)
		}
	}
	if before.SkewThresholds != nil || after.SkewThresholds != nil {
		thresholdOK := numericOK && before.SkewThresholds != nil && after.SkewThresholds != nil &&
			before.SkewThresholds.Source == after.SkewThresholds.Source
		for _, row := range []struct {
			name    string
			over095 bool
		}{
			{"偏度严格 >0.90（Worksheet 精确计数）", false}, {"偏度严格 >0.95（Worksheet 精确计数）", true},
		} {
			if before.SkewDistribution != nil && after.SkewDistribution != nil {
				break
			}
			beforeCount := reportWorksheetCount(before, row.over095)
			afterCount := reportWorksheetCount(after, row.over095)
			delta := "来源/几何未核验，不能比较"
			if thresholdOK {
				b := before.SkewThresholds.Above090
				a := after.SkewThresholds.Above090
				if row.over095 {
					b = before.SkewThresholds.Above095
					a = after.SkewThresholds.Above095
				}
				delta = fmt.Sprintf("%+d", a-b)
			}
			data.Measures = append(data.Measures, reportMeasure{row.name, beforeCount, afterCount, delta})
		}
	}
	if before.OQBound != nil || after.OQBound != nil {
		data.Measures = append(data.Measures, reportMeasure{"最小正交质量（Worksheet 实测范围）",
			reportOQBound(before), reportOQBound(after), "区间值，不能计算精确差"})
	}
	if referenceOK {
		data.Missing = append(data.Missing, "参考差值仅是原始指标的算术变化；至少一轮缺少完整几何身份核验，不可作为已核验的同几何优化效果")
	}
	if after.SkewDistribution == nil {
		data.NativeSource = "未取得"
		data.NativeCoverage = "无完整原生逐单元偏度分布"
		if after.SkewThresholds != nil {
			data.NativeSource = after.SkewThresholds.Source
			data.NativeCoverage = "Worksheet 精确阈值计数与 P95 实测范围；不具备完整分布"
			data.Missing = append(data.Missing, "已取得 >0.90、>0.95 实测计数和 P95 范围；未取得精确 P95、直方图与 Body 区域统计")
		} else {
			data.Missing = append(data.Missing, "目标轮未取得经过完整网格覆盖校验的原生逐单元偏度；P95、直方图和 Body 区域统计不可用")
		}
	} else {
		d := after.SkewDistribution
		data.NativeSource = d.Source
		data.NativeCoverage = fmt.Sprintf("完整覆盖 %d 个单元", d.SampleCount)
		if len(d.BodyStats) == 0 {
			data.Missing = append(data.Missing, "原生元素未能全部映射到有效 Body ID；区域统计未取得")
		}
		maxCount := int64(0)
		for _, b := range d.Bins {
			if b.Count > maxCount {
				maxCount = b.Count
			}
		}
		for i, b := range d.Bins {
			lowerBracket := "("
			if i == 0 {
				lowerBracket = "["
			}
			percent := float64(b.Count) / float64(d.SampleCount) * 100
			width := 0
			if maxCount > 0 {
				width = int(float64(b.Count) / float64(maxCount) * 100)
			}
			data.Histogram = append(data.Histogram, reportBin{Interval: fmt.Sprintf("%s%.2f, %.2f]", lowerBracket, b.Lower, b.Upper), Count: strconv.FormatInt(b.Count, 10), Portion: fmt.Sprintf("%.4f%%", percent), Width: width})
		}
	}
	// The exported report follows the same evidence-first visibility rule as
	// the native UI. Do not allocate columns solely to unacquired metrics.
	data.ShowOQ = before.OQMin != nil || after.OQMin != nil || before.OQBound != nil || after.OQBound != nil
	data.ShowP95 = p95b != nil || p95a != nil || before.SkewThresholds != nil || after.SkewThresholds != nil
	data.ShowHighSkew = data.ShowP95
	if len(history) == 0 {
		data.Missing = append(data.Missing, "无历史记录")
	}
	for _, p := range CompatibleQualityTrend(history, after) {
		q := p.Snapshot
		high90, high95 := "未取得", "未取得"
		if p.High090Fraction != nil {
			high90 = reportFraction(p.High090Fraction)
		}
		if p.High095Fraction != nil {
			high95 = reportFraction(p.High095Fraction)
		}
		p95Display := reportFloat(p.P95)
		if p.P95 == nil && q.SkewThresholds != nil {
			p95Display = reportWorksheetP95(q)
		}
		minOQ := reportFloat(p.OrthogonalMinimum)
		if p.OrthogonalMinimum == nil && q.OQBound != nil {
			minOQ = reportOQBound(q)
		}
		if q.SkewDistribution == nil && q.SkewThresholds != nil && q.MeshElements > 0 {
			high90 = fmt.Sprintf("%.3f%%", 100*float64(q.SkewThresholds.Above090)/float64(q.MeshElements))
			high95 = fmt.Sprintf("%.3f%%", 100*float64(q.SkewThresholds.Above095)/float64(q.MeshElements))
		}
		data.Trends = append(data.Trends, reportTrend{Time: q.CompletedAt.Format("2006-01-02 15:04:05"), JobID: q.JobID,
			MeshSignature: q.MeshSignature, MeshElements: reportInt(q.MeshElements, q.MeshElements > 0),
			MaxSkew: reportFloat(p.SkewMaximum), MeanSkew: reportFloat(p.SkewAverage), P95: p95Display,
			OrthogonalMin: minOQ, ElementQualityMin: reportFloat(p.ElementQualityMinimum), ElementQualityMean: reportFloat(p.ElementQualityAverage), High090: high90, High095: high95})
	}
	if len(data.Trends) < 2 {
		data.Missing = append(data.Missing, "同工程、同几何及同质量来源的趋势记录少于两轮")
	}
	for _, r := range QualityRegionComparisons(before, after) {
		highDelta, p95Delta := "不可比较/未取得", "不可比较/未取得"
		if r.Comparable {
			highDelta = fmt.Sprintf("%+d", *r.DeltaHigh095)
			p95Delta = reportDelta(r.DeltaP95)
		}
		b := r.Current
		data.Regions = append(data.Regions, reportRegion{ID: strconv.FormatInt(b.BodyID, 10), Count: strconv.FormatInt(b.Count, 10),
			Mean: fmt.Sprintf("%.5f", b.Mean), Maximum: fmt.Sprintf("%.5f", b.Max), P95: fmt.Sprintf("%.5f", b.P95),
			High090: strconv.FormatInt(b.Above090, 10), High095: strconv.FormatInt(b.Above095, 10), DeltaP95: p95Delta, DeltaHigh095: highDelta})
	}
	var output bytes.Buffer
	if err := analysisHTMLTemplate.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

// Offline report export only reads already persisted receipt snapshots. No
// ANSYS/ACT calls, no changes to existing Sizing, and no silent overwrite.
func ExportQualityAnalysisReport(w Workspace, history []QualitySnapshot, baseIndex, targetIndex int) (string, error) {
	html, err := BuildQualityAnalysisReport(history, baseIndex, targetIndex)
	if err != nil {
		return "", err
	}
	name := "quality_analysis_" + time.Now().Format("20060102_150405_000000000") + ".html"
	path := filepath.Join(w.Dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	if _, err = f.WriteString(html); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err = f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

var analysisHTMLTemplate = template.Must(template.New("quality_analysis").Funcs(template.FuncMap{
	"short": func(s string) string {
		if len(s) > 85 {
			return s[:85] + "…"
		}
		return s
	},
	"clean": func(s string) string { return strings.TrimSpace(s) },
}).Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>AutoMesh R8.1 网格质量分析报告</title><style>
body{font-family:"Microsoft YaHei UI","Segoe UI",sans-serif;margin:0;background:#f5f7fb;color:#20334d;line-height:1.65}main{max-width:1160px;margin:34px auto;padding:0 24px 50px}h1{font-size:26px;margin:0}h2{font-size:19px;margin:26px 0 12px}p,small{color:#54657a}.card{background:white;border:1px solid #e0e7f0;border-radius:12px;padding:20px 24px;margin-top:18px}.meta{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px 18px;overflow-wrap:anywhere}.notice{padding:12px 16px;background:#fff6ea;border-left:4px solid #c48032;border-radius:6px}.ok{background:#eff9f3;border-left-color:#278655}table{width:100%;border-collapse:collapse;font-size:13px;margin:12px 0}td,th{border-bottom:1px solid #e3e9f0;text-align:left;padding:8px;vertical-align:top;overflow-wrap:anywhere}th{background:#f4f7fb;color:#50647f}.overflow{overflow-x:auto}.bar{display:inline-block;min-height:12px;background:#2367d6;border-radius:3px}.high{background:#d18442}.mono{font-family:Consolas,monospace;font-size:12px;word-break:break-all}footer{margin-top:18px;color:#637184;font-size:12px}@media print{body{background:white}main{max-width:none;margin:0}.card{break-inside:avoid}}</style></head><body><main>
<h1>AutoMesh R8.1 · 网格质量分析报告</h1><p>导出时间：{{.Generated}} · 本地历史回执只读报告 · 历史记录 {{.HistoryCount}} 条</p>
<section class="card"><h2>一、数据身份与可比性</h2><p class="notice {{if .Comparable}}ok{{end}}">{{.Identity}}</p><div class="meta">
<div>工程：<span class="mono">{{.Project}}</span></div><div>Mesh 系统：{{.System}}</div>
<div>基准任务：<span class="mono">{{.BaseJob}}</span></div><div>目标任务：<span class="mono">{{.TargetJob}}</span></div>
<div>基准完成：{{.BaseTime}}</div><div>目标完成：{{.TargetTime}}</div>
<div>完整几何指纹：<span class="mono">{{.GeometryFingerprint}}</span></div><div>质量定义：{{.Definition}}</div>
<div>基准网格指纹：<span class="mono">{{.BeforeMesh}}</span></div><div>目标网格指纹：<span class="mono">{{.AfterMesh}}</span></div></div></section>
<section class="card"><h2>二、指标前后对比</h2><div class="overflow"><table><thead><tr><th>指标</th><th>基准</th><th>目标</th><th>{{if .ReferenceComparison}}参考差值（未核验）{{else}}变化（目标−基准）{{end}}</th></tr></thead><tbody>{{range .Measures}}<tr><td>{{.Name}}</td><td>{{.Before}}</td><td>{{.After}}</td><td>{{.Delta}}</td></tr>{{end}}</tbody></table></div><small>偏度越低越好，正交质量及 Element Quality 越高越好。几何身份或统计来源不一致时禁止宣称改善。</small></section>
<section class="card"><h2>三、同身份多轮趋势</h2><div class="overflow"><table><thead><tr><th>完成时间</th><th>单元数</th><th>最大偏度</th><th>平均偏度</th>{{if $.ShowP95}}<th>P95</th>{{end}}{{if $.ShowOQ}}<th>最小正交</th>{{end}}<th>最小 Element Quality</th><th>平均单元质量</th>{{if $.ShowHighSkew}}<th>&gt;0.90 比例</th><th>&gt;0.95 比例</th>{{end}}</tr></thead><tbody>{{range .Trends}}<tr><td>{{.Time}}<div class="mono">{{.JobID}}</div></td><td>{{.MeshElements}}</td><td>{{.MaxSkew}}</td><td>{{.MeanSkew}}</td>{{if $.ShowP95}}<td>{{.P95}}</td>{{end}}{{if $.ShowOQ}}<td>{{.OrthogonalMin}}</td>{{end}}<td>{{.ElementQualityMin}}</td><td>{{.ElementQualityMean}}</td>{{if $.ShowHighSkew}}<td>{{.High090}}</td><td>{{.High095}}</td>{{end}}</tr>{{else}}<tr><td colspan="10">缺少可比历史</td></tr>{{end}}</tbody></table></div><small>趋势按任务完成时间排序；每个指标只显示对应原生来源已核验的数据。</small></section>
<section class="card"><h2>四、目标轮偏度直方图</h2><p>原生逐单元来源：{{.NativeSource}}；覆盖情况：{{.NativeCoverage}}</p><table><thead><tr><th>偏度区间</th><th>单元数</th><th>占比</th><th>相对数量</th></tr></thead><tbody>{{range .Histogram}}<tr><td>{{.Interval}}</td><td>{{.Count}}</td><td>{{.Portion}}</td><td><span class="bar" style="width:{{.Width}}%"></span></td></tr>{{else}}<tr><td colspan="4">未取得真实偏度分布，禁止推算</td></tr>{{end}}</tbody></table><small>P95 为最近秩统计；高偏度计数严格采用 &gt;0.90 和 &gt;0.95。</small></section>
<section class="card"><h2>五、Body 区域统计及前后对比</h2><div class="overflow"><table><thead><tr><th>Body ID</th><th>单元数</th><th>平均偏度</th><th>最大偏度</th><th>P95</th><th>&gt;0.90</th><th>&gt;0.95</th><th>Δ P95</th><th>Δ &gt;0.95</th></tr></thead><tbody>{{range .Regions}}<tr><td>{{.ID}}</td><td>{{.Count}}</td><td>{{.Mean}}</td><td>{{.Maximum}}</td><td>{{.P95}}</td><td>{{.High090}}</td><td>{{.High095}}</td><td>{{.DeltaP95}}</td><td>{{.DeltaHigh095}}</td></tr>{{else}}<tr><td colspan="9">Body 身份未完整取得，无法出具区域统计</td></tr>{{end}}</tbody></table></div></section>
<section class="card"><h2>六、数据来源与未验收项目</h2><div class="meta"><div>质量回执来源：{{.QualitySource}}</div><div>偏度采集：{{.SkewSource}}</div><div>正交质量采集：{{.OQSource}}</div><div>Element Quality 采集：{{.EQSource}}</div><div>原生偏度分布：{{.NativeSource}}</div></div>{{range .Missing}}<p class="notice">{{.}}</p>{{end}}
<p>Windows 原生交互、ANSYS ACT 逐单元 SKEWNESS 可用性与 Body 对应关系均需实机验收。本报告仅反映本地真实持久化回执，不是实机验收证书。</p></section>
<footer>AutoMesh R8.1 · 报告只读生成，不执行 Workbench/ANSYS、GenerateMesh 或 Sizing 修改。</footer></main></body></html>`))
