package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"
)

const htmlReport = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>AutoMesh 质量报告</title><style>
:root{font-family:"Microsoft YaHei UI", "Segoe UI",sans-serif;color:#20304c;background:#f5f7fb}
*{box-sizing:border-box}body{margin:0;padding:44px 26px}.page{max-width:1050px;margin:auto}
.header{display:flex;justify-content:space-between;align-items:center;margin-bottom:26px}h1{font-size:28px;margin:0}small{color:#7a879c}
.banner{padding:24px 28px;border-radius:20px;background:#edf4ff;margin-bottom:18px}.banner strong{font-size:22px}.banner p{margin-bottom:0;color:#61728d}
.grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-bottom:16px}.card{background:white;border:1px solid #e5ebf5;border-radius:17px;padding:20px;min-width:0}
.k{font-size:13px;color:#72829c;margin-bottom:10px}.v{font-size:23px;font-weight:700;word-break:break-word}.wide{margin:16px 0}h2{font-size:18px}.desc{line-height:1.8;word-break:break-all;white-space:pre-wrap;font-size:13px;color:#566783}
.status{color:#195bcb}.warning{color:#a36b15}.footer{font-size:12px;color:#7a879c;margin-top:25px}table{width:100%;border-collapse:collapse;font-size:13px}th,td{text-align:left;border-bottom:1px solid #eef0f4;padding:9px 8px}th{color:#697791;font-weight:600}.subtle{color:#7c879c;font-size:13px}
@media(max-width:750px){.grid{grid-template-columns:1fr}body{padding:20px 14px}}
</style></head><body><main class="page"><div class="header"><h1>AutoMesh · 网格质量报告</h1><small>{{.Date}}</small></div>
<div class="banner"><strong>任务：{{.Phase}} · {{.Status}}</strong><p>数据来自当前 ANSYS Meshing 的任务返回；不以估算值替代未获取的质量指标。</p><p><strong>{{.QualityNote}}</strong></p></div>
<div class="grid">
<div class="card"><div class="k">网格单元数</div><div class="v">{{.Elements}}</div></div>
<div class="card"><div class="k">最大偏度</div><div class="v">{{.MaxSkew}}</div></div>
<div class="card"><div class="k">正交质量最小值</div><div class="v">{{.MinOQ}}</div></div>
<div class="card"><div class="k">Worksheet 失败单元</div><div class="v">{{.FailedCount}}</div></div>
<div class="card"><div class="k">Worksheet 警告单元</div><div class="v">{{.WarningCount}}</div></div>
<div class="card"><div class="k">CFD 质量判断</div><div class="v status">{{.Gate}}</div></div>
</div>
<div class="card wide"><h2>正交质量读取状态与说明</h2>
<div class="desc">{{.OrthoDetail}}</div>
<p class="subtle">ANSYS Meshing 图形界面提供 Orthogonal Quality，并不保证此 ACT 会话具有同名 MeshMetricType 枚举。没有返回实测值时不做推算，也不启动 Fluent。</p></div>
<div class="card wide"><h2>分级诊断与操作建议</h2>
<div class="desc">{{.QualityAdvice}}</div><p class="subtle">以下仅为基于可读取指标的筛查意见，不代表 Fluent 验收或网格无关性证明。</p></div>
<div class="card wide"><h2>实测补充统计</h2>
<div class="desc">网格节点数：{{.Nodes}}
Skewness 最小 / 平均 / 最大：{{.SkewMin}} / {{.SkewMean}} / {{.MaxSkew}}
质量数据来源：{{.MeasurementSource}}
提醒：以上为当前 ANSYS ACT 返回值，不代表 Fluent 中的质量、边界条件和收敛性已验证。</div>
</div>
<div class="card wide"><h2>质量数据说明</h2>
<p>单元数、偏度等来自当前 ACT 返回；未返回的项目不会使用推算值代替。<strong>Worksheet 显示 0 仅代表当前已计算的指标和当前警告/失败阈值</strong>，不能证明整个网格没有低质量单元。最小正交质量如 ACT 不可读取，需要在 Fluent 中复核。</p>
{{if .Worksheet}}<table><thead><tr><th>Worksheet 指标</th><th>失败</th><th>警告</th></tr></thead><tbody>{{range .Worksheet}}<tr><td>{{.Name}}</td><td>{{.Failed}}</td><td>{{.Warning}}</td></tr>{{end}}</tbody></table>{{else}}<p>Worksheet 尚未返回可核验的失败/警告数据，请在 Meshing 中启用并刷新 Quality Worksheet。</p>{{end}}
</div>
{{if .OfficialMark}}<div class="card wide"><h2>ANSYS Worksheet 元素定位操作</h2>
<div class="desc">{{.OfficialStatus}}
{{.OfficialMessage}}
新增原生选择组：{{.OfficialGroups}}
Worksheet 阈值内目标元素数：{{.OfficialCount}}
ACT 可读元素 ID 样本：{{.OfficialSample}}</div>
<p>如无新增选择组，请检查 Worksheet 是否已计算、该阈值下是否确有目标元素；不会用采样元素伪造原生失败组。</p>
</div>{{end}}
{{if .RiskRequested}}<div class="card wide" id="risk" style="border:2px solid #d7e5ff"><h2>风险单元定位结果</h2>
<p><strong>{{.RiskHeadline}}</strong></p>
<p class="subtle">{{.RiskDescription}}</p>
<div class="desc">采样状态：{{.RiskStatus}} · 网格总单元 {{.Elements}} · 已检查 {{.RiskChecked}} · 采样覆盖率 {{.Coverage}} · 样本风险单元 {{.RiskCount}}</div>
{{if .Worksheet}}<h2>ANSYS Mesh Quality Worksheet 官方失败计数</h2><table><thead><tr><th>指标</th><th>失败单元</th><th>警告单元</th></tr></thead><tbody>{{range .Worksheet}}<tr><td>{{.Name}}</td><td>{{.Failed}}</td><td>{{.Warning}}</td></tr>{{end}}</tbody></table>{{else}}<p class="subtle">官方失败单元计数暂未获得；采样结果不能代替 ANSYS 的 Skewness / Orthogonal Quality。</p>{{end}}
{{if .ElementSamples}}<h2>采样定位清单（最多 25 条）</h2><p><a href="risk_samples.csv">导出定位 CSV：单元 ID、形状代理值、中心坐标</a></p>
<table><thead><tr><th>单元 ID</th><th>形状代理值</th><th>中心坐标（网格单位）</th></tr></thead><tbody>{{range .ElementSamples}}<tr><td>{{.ID}}</td><td>{{.Score}}</td><td>{{.XYZ}}</td></tr>{{end}}</tbody></table>
{{else}}<p>此次没有返回可定位的四面体风险样本。可能是采样未命中、网格不含可读取的四面体或接口不可用，<strong>不代表整个网格不存在坏单元</strong>。</p>{{end}}
<p class="subtle">若官方计数显示失败单元，返回 AutoMesh 点击「标记失败单元」；ANSYS 支持时将在 Meshing 创建原生失败元素选择组。此操作需人工确认，不能自动将采样 ID 当作官方失败元素。</p></div>{{end}}
<div class="card wide"><h2>项目和设置</h2><div class="desc">项目：{{.Project}}
几何单位：{{.Unit}}
几何体/面：{{.Bodies}} / {{.Faces}}
局部 Sizing 新增：{{.AddedControls}}
异常恢复复用原控制：{{.ReusedControls}}
局部细化面积代理预算：{{.BudgetProxy}}
网格导出状态：{{.Export}}
网格生成阶段耗时：{{.Seconds}}
整个任务耗时：{{.WallSeconds}}
任务期间最低系统可用内存：{{.MinFreeRAM}}
网格参考状态：{{.MeshReference}}</div></div>
{{if .Capabilities}}<div class="card wide"><h2>ANSYS 2024 R2 实际接口探针</h2><p class="subtle">仅报告已加载 ACT 会话中的接口可见性，不代表每个算法在本模型上都能正确执行。</p><table><thead><tr><th>能力</th><th>检测结果</th></tr></thead><tbody>{{range .Capabilities}}<tr><td>{{.Name}}</td><td>{{.Value}}</td></tr>{{end}}</tbody></table></div>{{end}}
{{if .Proposed}}<div class="card wide"><h2>按面积分组的候选局部细化方案（尚未应用）</h2><p class="subtle">按几何面面积和面型分类，并不证明属于晶圆、孔口或导流件。请在 Meshing 中通过命名组核查，调整或关闭不需要细化的组。</p><table><thead><tr><th>细化类别</th><th>建议尺寸</th><th>候选面数</th><th>识别置信度</th></tr></thead><tbody>{{range .Proposed}}<tr><td>{{.Name}}</td><td>{{.MM}}</td><td>{{.Faces}}</td><td>{{.Mode}}</td></tr>{{end}}</tbody></table></div>{{end}}
{{if .EditableGroups}}<div class="card wide"><h2>已绑定 Sizing 的 Meshing 命名区域（未执行生成）</h2><p class="subtle">每行对应一个真实的 Named Selection 和 Sizing，未重新选面。既有 Sizing 保留用户手动修改的尺寸。</p><table><thead><tr><th>Named Selection / Sizing</th><th>尺寸</th><th>面数</th><th>创建状态</th></tr></thead><tbody>{{range .EditableGroups}}<tr><td>{{.Name}}</td><td>{{.MM}}</td><td>{{.Faces}}</td><td>{{.Mode}}</td></tr>{{end}}</tbody></table></div>{{end}}
{{if .NativeControls}}<div class="card wide"><h2>ANSYS 原生局部控制生效记录</h2><p class="subtle">“已尝试”不等于实测网格层数符合预期；以 ACT 返回和实际网格结果为准。</p><table><thead><tr><th>控制名称</th><th>局部尺寸</th><th>涉及面数</th><th>实际执行模式</th></tr></thead><tbody>{{range .NativeControls}}<tr><td>{{.Name}}</td><td>{{.MM}}</td><td>{{.Faces}}</td><td>{{.Mode}}</td></tr>{{end}}</tbody></table></div>{{end}}
<div class="card wide"><h2>质量限制与下一步</h2><div class="desc">{{.Action}}
{{.Warnings}}</div></div>
<div class="footer">AutoMesh 2024 R2 R6.2 · 软件仅提供网格自动化与初步诊断；最终需要在 Fluent 中检查网格与边界条件。</div>
</main></body></html>`

type diagnosticRow struct{ ID, Score, XYZ string }
type capabilityRow struct{ Name, Value string }
type worksheetRow struct{ Name, Failed, Warning string }
type sizingRow struct{ Name, MM, Faces, Mode string }
type htmlFields struct {
	OfficialMark                                                                                                                                                                                   bool
	OfficialStatus, OfficialMessage, OfficialGroups, OfficialCount, OfficialSample                                                                                                                 string
	RiskRequested                                                                                                                                                                                  bool
	RiskHeadline, RiskDescription, RiskStatus, RiskChecked, RiskCount                                                                                                                              string
	QualityNote, OrthoDetail                                                                                                                                                                       string
	QualityAdvice, MinFreeRAM, MeshReference                                                                                                                                                       string
	FailedCount, WarningCount, Nodes, SkewMin, SkewMean, MeasurementSource                                                                                                                         string
	Worksheet                                                                                                                                                                                      []worksheetRow
	Coverage                                                                                                                                                                                       string
	ElementSamples                                                                                                                                                                                 []diagnosticRow
	NativeControls                                                                                                                                                                                 []sizingRow
	Proposed                                                                                                                                                                                       []sizingRow
	EditableGroups                                                                                                                                                                                 []sizingRow
	Capabilities                                                                                                                                                                                   []capabilityRow
	Date, Phase, Status, Elements, MaxSkew, MinOQ, Gaps, Cylinders, Gate, Project, Unit, Bodies, Faces, AddedControls, ReusedControls, BudgetProxy, Seconds, WallSeconds, Action, Warnings, Export string
}

func stringMetric(v any) string {
	if v == nil {
		return "未获取"
	}
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(n), 'f', -1, 32)
	}
	return fmt.Sprint(v)
}
func nested(m map[string]any, names ...string) any {
	var v any = m
	for _, n := range names {
		mm, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = mm[n]
	}
	return v
}
func (w Workspace) SaveHTMLReport(s *Snapshot, r Result) (string, error) {
	m := map[string]any{}
	if len(r.Summary) > 0 {
		if err := json.Unmarshal(r.Summary, &m); err != nil {
			return "", err
		}
	}
	qualityMeasured := nested(m, "quality") != nil
	qualityNote := "本任务仅操作命名或局部设置，没有检查网格质量。请回到「质量报告」点击「只读检查网格质量」。"
	if qualityMeasured {
		qualityNote = "本次质量数据来自 ANSYS Meshing 当前会话；未提供的质量指标会明确保留为「未获取」。"
	} else if r.Phase == "quality_check" && m["mesh_elements"] == float64(0) {
		qualityNote = "当前 Meshing 中没有网格，无法测量质量。"
	}
	f := htmlFields{Date: time.Now().Format("2006-01-02 15:04"), Phase: r.Phase, Status: r.Status,
		QualityNote: qualityNote, FailedCount: "未获取", WarningCount: "未获取",
		Nodes: stringMetric(nested(m, "quality", "nodes")), SkewMin: stringMetric(nested(m, "quality", "skewness", "min")),
		SkewMean: stringMetric(nested(m, "quality", "skewness", "mean")), MeasurementSource: stringMetric(m["quality_source"]),
		Elements: stringMetric(m["mesh_elements"]), MaxSkew: stringMetric(nested(m, "quality", "skewness", "max")),
		MinOQ:     stringMetric(nested(m, "quality", "orthogonal_quality", "min")),
		Gaps:      stringMetric(nested(m, "geometry_evidence", "candidate_groups_published")),
		Cylinders: stringMetric(nested(m, "geometry_evidence", "valid_area_faces")),
		Gate:      stringMetric(m["quality_gate"]) + " / " + stringMetric(m["quality_status"]), AddedControls: stringMetric(m["added_control_count"]),
		ReusedControls: stringMetric(m["reused_control_count"]),
		Seconds:        stringMetric(m["seconds"]), Action: stringMetric(m["next_action"]), Export: stringMetric(m["export"]),
	}
	f.OrthoDetail = "本任务尚未取得正交质量诊断；先执行「只读检查网格质量」。"
	if qualityMeasured {
		diag, _ := nested(m, "quality", "orthogonal_quality_diagnostic").(map[string]any)
		if nested(m, "quality", "orthogonal_quality", "min") != nil {
			f.OrthoDetail = "ACT 当前网格原生实测。读取路径：MeshMetricType.OrthogonalQuality；请在 Fluent 中按实际边界拓扑复核。"
		} else {
			switch stringMetric(diag["status"]) {
			case "enum_not_exposed":
				f.OrthoDetail = "当前 ANSYS ACT 会话的 MeshMetricType 不包含 OrthogonalQuality，因此 AutoMesh 无法直接读取官方最小正交质量。这不是质量为 0，也不是网格自动失效。需要在 Meshing 原生统计或 Fluent Mesh Quality 中检查。"
			case "metric_read_failed":
				f.OrthoDetail = "ACT 已暴露 OrthogonalQuality，但取数或赋值失败。异常：" + stringMetric(diag["reason"])
			default:
				f.OrthoDetail = "当前任务未获得有效正交质量；请检查 ACT 质量接口状态。禁止以 1-最大偏度估算正交质量。"
			}
		}
	}
	f.QualityAdvice = qualityAdvice(m)
	f.MinFreeRAM = "未记录"
	f.MeshReference = stringMetric(nested(m, "mesh_reference", "status"))
	if !qualityMeasured {
		f.Nodes = "本任务未测量"
		f.SkewMin = "本任务未测量"
		f.SkewMean = "本任务未测量"
		f.MeasurementSource = "无；本任务不是质量检查"
		f.MaxSkew = "本任务未测量"
		f.MinOQ = "本任务未测量"
		f.Gate = "尚未进行质量检查"
	} else if nested(m, "quality", "orthogonal_quality", "min") == nil {
		f.MinOQ = "ANSYS ACT 未提供；需 Fluent 核验"
	}
	if qualityMeasured && f.MeasurementSource == "未获取" {
		f.MeasurementSource = "ANSYS ACT 当前网格 MeshMetric"
	}
	if f.Elements == "未获取" && s != nil && s.Mesh != nil {
		// Snapshot values are a different source, and may predate a later Mesh edit.
		if elem, ok := s.Mesh["Elements"]; ok {
			f.Elements = stringMetric(elem) + "（扫描时）"
		}
	}
	if budget, ok := nested(m, "geometry_evidence", "resource_preflight").(map[string]any); ok {
		f.BudgetProxy = fmt.Sprintf("%v / %v（面积/尺寸²代理，非实际内存或体网格数量）", budget["surface_proxy"], budget["limit"])
	} else if budget, ok := m["resource_preflight"].(map[string]any); ok {
		f.BudgetProxy = fmt.Sprintf("%v / %v（面积/尺寸²代理，非内存预测）", budget["surface_proxy"], budget["limit"])
	} else {
		f.BudgetProxy = "本阶段未执行资源预算或不可读取"
	}
	if clock, err := readJSON[map[string]any](w.path("last_task_timing.json")); err == nil && stringMetric(clock["job_id"]) == r.JobID && r.JobID != "" {
		f.WallSeconds = stringMetric(clock["elapsed_seconds"]) + " 秒"
		if clock["min_system_free_ram_gib"] != nil {
			f.MinFreeRAM = stringMetric(clock["min_system_free_ram_gib"]) + " GiB（系统剩余内存，非任务峰值）"
		}
	} else {
		f.WallSeconds = "未测量（无同任务记录）"
	}
	if caps, ok := m["capabilities"].(map[string]any); ok {
		keys := make([]string, 0, len(caps))
		for k := range caps {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := "未验证"
			switch x := caps[key].(type) {
			case bool:
				if x {
					val = "接口存在（待实机执行验证）"
				} else {
					val = "不可用或未暴露"
				}
			case string:
				val = x
			}
			f.Capabilities = append(f.Capabilities, capabilityRow{Name: key, Value: val})
		}
	}
	if items, ok := m["proposed_sizing"].([]any); ok {
		for _, item := range items {
			if obj, ok := item.(map[string]any); ok {
				kinds := map[string]string{"area_small": "小面积候选", "area_repeated": "重复面积候选", "area_review": "其他大面（审核）", "named_existing": "用户已有命名"}
				surfaces := map[string]string{"Plane": "平面", "Cylinder": "圆柱面", "AnalyticCurve": "解析曲面", "Freeform": "自由曲面", "Other": "其他", "UserSelection": "已有命名"}
				rawKind := stringMetric(obj["category"])
				name := rawKind
				if label, ok := kinds[rawKind]; ok {
					name = label
				}
				fam := stringMetric(obj["surface_family"])
				if label, ok := surfaces[fam]; ok {
					name += " / " + label
				}
				if obj["area_mm2_min"] != nil {
					name += "（面积 " + stringMetric(obj["area_mm2_min"]) + "–" + stringMetric(obj["area_mm2_max"]) + " mm²）"
				}
				f.Proposed = append(f.Proposed, sizingRow{
					Name: name, MM: stringMetric(obj["size_mm"]) + " mm",
					Faces: stringMetric(obj["face_count"]), Mode: stringMetric(obj["confidence"]),
				})
			}
		}
	}
	if items, ok := m["editable_named_groups"].([]any); ok {
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				f.EditableGroups = append(f.EditableGroups, sizingRow{
					Name: stringMetric(item["named_selection"]) + " / " + stringMetric(item["sizing"]),
					MM:   stringMetric(item["size_mm"]) + " mm", Faces: stringMetric(item["face_count"]),
					Mode: stringMetric(item["named_status"]) + " / " + stringMetric(item["sizing_status"]),
				})
			}
		}
	}
	if items, ok := m["existing_named_sizing"].([]any); ok {
		f.AddedControls = stringMetric(m["created_sizing_count"])
		f.ReusedControls = stringMetric(m["preserved_sizing_count"])
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				f.EditableGroups = append(f.EditableGroups, sizingRow{
					Name: stringMetric(item["named_selection"]) + " → " + stringMetric(item["sizing"]),
					MM:   stringMetric(item["size_mm"]) + " mm", Faces: stringMetric(item["face_count"]),
					Mode: stringMetric(item["sizing_status"]),
				})
			}
		}
		f.Action = "已用现有 Named Selection 绑定 Sizing；请到 Meshing 树中复核作用范围。原有命名未改、网格未主动清除或生成。"
	}
	if items, ok := m["configured"].([]any); ok {
		for _, item := range items {
			if obj, ok := item.(map[string]any); ok {
				mode := "未获取"
				if setting, ok := obj["native_sizing"].(map[string]any); ok {
					mode = stringMetric(setting["applied"])
					if reason, ok := setting["reason"].(string); ok {
						mode += "（" + reason + "）"
					}
				}
				f.NativeControls = append(f.NativeControls, sizingRow{
					Name: stringMetric(obj["name"]), MM: stringMetric(obj["mm"]) + " mm",
					Faces: stringMetric(obj["face_count"]), Mode: mode,
				})
			}
		}
	}
	if m["output"] != nil {
		f.Export = stringMetric(m["output"]) + "（需 Fluent 复核）"
	}
	if s != nil {
		f.Project = s.Project
		f.Unit = "Meshing显示 " + s.DisplayLengthUnit + " / CAD底层 " + s.GeometryUnit + "（所有输入尺寸统一 mm）"
		f.Bodies = fmt.Sprint(len(s.Bodies))
		f.Faces = fmt.Sprint(len(s.Faces))
	}
	if len(r.Error) > 0 {
		f.Warnings = r.Error
	} else if warnings, ok := nested(m, "quality", "warnings").([]any); ok {
		for _, warning := range warnings {
			f.Warnings += fmt.Sprint(warning) + "\n"
		}
	}
	if r.Phase == "mark_failed" || r.Phase == "mark_warning" {
		f.OfficialMark = true
		f.OfficialStatus = stringMetric(m["status"])
		f.OfficialMessage = stringMetric(m["message"])
		f.OfficialCount = stringMetric(m["worksheet_count"])
		f.OfficialGroups = stringMetric(m["created_groups"])
		f.OfficialSample = stringMetric(m["selected_element_ids_sample"])
	}
	f.RiskRequested = r.Phase == "diagnose"
	probe := nested(m, "element_probe")
	if po, ok := probe.(map[string]any); ok {
		f.RiskStatus = stringMetric(po["status"])
		f.RiskChecked = stringMetric(po["checked_element_count"])
		if status := stringMetric(po["error"]); status != "未获取" {
			f.RiskDescription = "诊断 API 返回：" + status
		}
		f.Coverage = stringMetric(po["sampling_fraction"])
		if rows, ok := po["samples"].([]any); ok {
			for _, r := range rows {
				if item, ok := r.(map[string]any); ok {
					f.ElementSamples = append(f.ElementSamples, diagnosticRow{
						ID: stringMetric(item["element_id"]), Score: stringMetric(item["shape_proxy"]),
						XYZ: stringMetric(item["centroid_mesh_units"]),
					})
				}
			}
		}
		if warning, ok := po["warning"].(string); ok {
			f.Warnings += "\n" + warning
		}
	}
	if mqs, ok := m["worksheet_diagnostics"].(map[string]any); ok {
		failure, warning := 0, 0
		counted := false
		if ms, ok := mqs["metrics"].([]any); ok {
			for _, raw := range ms {
				mr, ok := raw.(map[string]any)
				if !ok || mr["failed_count"] == nil || mr["warning_count"] == nil {
					continue
				}
				f.Worksheet = append(f.Worksheet, worksheetRow{Name: stringMetric(mr["name"]),
					Failed: stringMetric(mr["failed_count"]), Warning: stringMetric(mr["warning_count"])})
				if v, ok := mr["failed_count"].(float64); ok {
					failure += int(v)
				}
				if v, ok := mr["warning_count"].(float64); ok {
					warning += int(v)
				}
				counted = true
			}
		}
		if counted {
			f.FailedCount = fmt.Sprint(failure) + "（跨指标合计，非去重单元）"
			f.WarningCount = fmt.Sprint(warning) + "（跨指标合计，非去重单元）"
			f.Warnings += "\nWorksheet 计数按不同质量指标累加，同一单元可能重复出现；应查看下方逐指标明细，不能按合计判断唯一坏单元数。"
		}
	}
	if qualityMeasured && s != nil {
		if project, ok := m["project"].(string); ok && project != "" && project != s.Project {
			f.QualityNote = "警告：报告工程路径与当前快照不匹配，不能当作当前网格质量。"
		} else if fingerprint, ok := m["geometry_fingerprint"].(string); ok && fingerprint != "" && fingerprint != s.Fingerprint {
			f.QualityNote = "警告：报告几何指纹与当前快照不一致，属于历史结果。"
		}
	}
	if f.RiskRequested {
		f.RiskCount = fmt.Sprint(len(f.ElementSamples))
		f.RiskHeadline = "已完成风险单元采样定位，但不代表完整网格已排查"
		if len(f.ElementSamples) == 0 {
			f.RiskHeadline = "本次未获得风险单元位置；不代表不存在失败单元"
		}
		if f.RiskDescription == "" {
			f.RiskDescription = "本次仅检查部分四面体几何形状，不是全网格检索；形状代理值低于 0.20 的采样元素才列出。请用官方质量统计复核。"
		}

	}
	evidence := nested(m, "geometry_evidence")
	if ev, ok := evidence.(map[string]any); ok {
		if n, ok := ev["repeated_passage_pairs"]; ok {
			f.Warnings += "\n重复狭缝候选配对数：" + stringMetric(n) + "（连通性待验证）"
		}
		if warnings, ok := ev["warnings"].([]any); ok {
			for _, warning := range warnings {
				f.Warnings += "\n局部细化提示：" + stringMetric(warning)
			}
		}
	}
	if repair, ok := nested(m, "repair_scope").(map[string]any); ok {
		if reason, ok := repair["reason"].(string); ok {
			f.Warnings += "\n局部重划评估：" + reason
		}
	}
	if f.Action == "未获取" {
		if r.Phase == "prepare" {
			f.Action = "已发布可编辑局部控制；请先在 ANSYS Meshing 树内高亮检查和修改 Named Selection / Sizing，再点击审核后生成。"
		} else {
			f.Action = "自动网格不能单凭偏度证明 Fluent 适用性；请复核正交质量与边界命名。"
		}
	}
	var buf bytes.Buffer
	t, err := template.New("report").Parse(htmlReport)
	if err != nil {
		return "", err
	}
	if err := t.Execute(&buf, f); err != nil {
		return "", err
	}
	if f.RiskRequested {
		var rows bytes.Buffer
		csvWriter := csv.NewWriter(&rows)
		if err := csvWriter.Write([]string{"element_id", "shape_proxy_not_official", "centroid_mesh_units"}); err != nil {
			return "", err
		}
		for _, sample := range f.ElementSamples {
			if err := csvWriter.Write([]string{sample.ID, sample.Score, sample.XYZ}); err != nil {
				return "", err
			}
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return "", err
		}
		if err := writeAtomic(w.path("risk_samples.csv"), rows.Bytes()); err != nil {
			return "", err
		}
	}
	// Publishing names or Sizing must not erase the last ACT quality report.
	// Its own activity report remains separately accessible for troubleshooting.
	name := "activity_report.html"
	if qualityMeasured || r.Phase == "quality_check" || r.Phase == "diagnose" ||
		r.Phase == "generate_reviewed" || r.Phase == "auto" || r.Phase == "auto_onepass" {
		name = "quality_report.html"
	}
	path := w.path(name)
	return path, writeAtomic(path, buf.Bytes())
}

// qualityAdvice deliberately avoids claiming physical cause or Fluent readiness.
// Without full ACT quality distributions, the correct result is an actionable
// checklist rather than invented worst-cell positions.
func qualityAdvice(summary map[string]any) string {
	quality, ok := summary["quality"].(map[string]any)
	if !ok || len(quality) == 0 {
		return "没有本次质量测量。先执行『只读检查网格质量』；不要凭上次网格截图判断。"
	}
	parse := func(path ...string) (float64, bool) {
		raw := nested(summary, path...)
		switch x := raw.(type) {
		case float64:
			return x, true
		case string:
			v, e := strconv.ParseFloat(x, 64)
			return v, e == nil
		}
		return 0, false
	}
	skew, hasSkew := parse("quality", "skewness", "max")
	oq, hasOQ := parse("quality", "orthogonal_quality", "min")
	messages := []string{}
	if !hasSkew {
		messages = append(messages, "1. 最大偏度未获得：到 Meshing Quality Worksheet 刷新 Skewness，并在 Fluent 再核对。")
	} else if skew > 0.95 {
		messages = append(messages, fmt.Sprintf("1. 最大偏度 %.4f > 0.95（筛查值）：先定位超限单元和几何面，检查薄壁/狭缝/孔缘，再仅修改相关局部 Sizing。", skew))
	} else if skew >= 0.85 {
		messages = append(messages, fmt.Sprintf("1. 最大偏度 %.4f：属于需要检查的较高范围，重点定位 Worksheet 警告区；不能只凭最大值断言全部合格。", skew))
	} else {
		messages = append(messages, fmt.Sprintf("1. 最大偏度 %.4f：初步未触及 0.95 筛查值，但仍需查看质量分布和局部差网格。", skew))
	}
	if !hasOQ {
		messages = append(messages, "2. 正交质量未返回：无法完成 CFD 网格质量判断；使用 Fluent Mesh Check 获取最小值、低值分布和异常位置。")
	} else if oq < 0.1 {
		messages = append(messages, fmt.Sprintf("2. 最小正交质量 %.4f < 0.1（筛查值）：优先检查该低质量单元位置，必要时调整几何或尺寸。", oq))
	} else {
		messages = append(messages, fmt.Sprintf("2. 最小正交质量 %.4f：仍需核对低值单元占比，而不是只看一个极值。", oq))
	}
	if ws, ok := summary["worksheet_diagnostics"].(map[string]any); ok {
		if rows, ok := ws["metrics"].([]any); ok && len(rows) > 0 {
			messages = append(messages, "3. 逐项检查下方 Worksheet 的失败/警告计数；若不为零，使用『标记失败/警告单元』生成原生选择组，高亮后才判断缺陷来源。")
		} else {
			messages = append(messages, "3. Worksheet 未返回可核验计数：先在 Meshing 刷新质量表，不能把缺失当成零。")
		}
	} else {
		messages = append(messages, "3. 未获得 Worksheet 指标计数：刷新 Meshing 质量表，必要时执行辅助诊断定位。")
	}
	messages = append(messages, "4. 记录同方案的单元数、细化区域、生成耗时和可用内存；仅在质量异常区域进行下一轮修改，最终用 Fluent 检查边界、守恒和网格无关性。")
	return strings.Join(messages, "\n")
}
