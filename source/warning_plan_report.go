package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// WarningPlan is a read-only draft; nothing here executes an ANSYS action.
type WarningPlan struct {
	JobID          string             `json:"job_id,omitempty"`
	SchemaVersion  int                `json:"schema_version"`
	Status         string             `json:"status"`
	PlanID         string             `json:"plan_id"`
	Project        string             `json:"project"`
	Fingerprint    string             `json:"geometry_fingerprint"`
	WarningCount   int                `json:"warning_count"`
	CoveredCount   int                `json:"covered_count"`
	UncoveredCount int                `json:"uncovered_count"`
	MeshElements   int                `json:"mesh_elements"`
	TotalCellProxy int                `json:"total_cell_proxy"`
	QualityMetric  string             `json:"quality_metric,omitempty"`
	Clusters       []WarningHotspot   `json:"clusters"`
	Notes          string             `json:"notes"`
	RejectedGroups []WarningRejection `json:"rejected_groups,omitempty"`
}
type WarningRejection struct {
	ElementIDs       []int   `json:"element_ids"`
	Reason           string  `json:"reason"`
	ProposedRadiusMM float64 `json:"proposed_radius_mm,omitempty"`
	Count            int     `json:"count,omitempty"`
	Limit            int     `json:"limit,omitempty"`
	Proxy            int     `json:"cell_budget_proxy,omitempty"`
}
type WarningHotspot struct {
	ID            string    `json:"candidate_id"`
	SkewMax       *float64  `json:"skewness_max,omitempty"`
	SkewMean      *float64  `json:"skewness_mean,omitempty"`
	SkewVerified  bool      `json:"skewness_verified"`
	SkewSource    string    `json:"skewness_source"`
	EQMin         *float64  `json:"element_quality_min,omitempty"`
	EQMean        *float64  `json:"element_quality_mean,omitempty"`
	EQVerified    bool      `json:"element_quality_verified"`
	EQSource      string    `json:"element_quality_source,omitempty"`
	BatchIndex    int       `json:"batch_index"`
	BodyID        int       `json:"body_id"`
	CentreMM      []float64 `json:"centre_mm"`
	RadiusMM      float64   `json:"radius_mm"`
	SizeMM        float64   `json:"element_size_mm"`
	ElementIDs    []int     `json:"element_ids"`
	Proxy         int       `json:"cell_budget_proxy"`
	ScopeKind     string    `json:"scope_kind"`
	Mapping       []string  `json:"mapping_methods"`
	MappedFaceIDs []int     `json:"mapped_face_ids,omitempty"`
	WaferGap      string    `json:"wafer_gap_clearance"`
}

type warningPlanReportData struct {
	WarningPlan
	ExistingControls      []warningControlSnapshot
	ExistingControlsKnown bool
	ExistingBudget        int
	BudgetView            budgetDiagnosticView
	FeatureView           []smallFeatureCandidateView
}

func validateWarningDiagnostic(p WarningPlan, snapshot *Snapshot, reviewedPlanID string) error {
	if snapshot == nil || p.SchemaVersion != 1 || p.PlanID == "" || p.PlanID != reviewedPlanID || p.Project != snapshot.Project || p.Fingerprint != snapshot.Fingerprint {
		return fmt.Errorf("警告区方案与当前工程或几何不一致")
	}
	if p.WarningCount < 1 || p.CoveredCount < 0 || p.UncoveredCount < 0 || p.CoveredCount+p.UncoveredCount != p.WarningCount || len(p.Clusters) > 4096 {
		return fmt.Errorf("警告区诊断计数无效")
	}
	if p.Status == "no_safe_candidates" {
		if len(p.Clusters) != 0 || p.CoveredCount != 0 || p.UncoveredCount != p.WarningCount {
			return fmt.Errorf("无候选方案统计不一致")
		}
		return nil
	}
	if p.Status != "ready" || len(p.Clusters) == 0 || p.CoveredCount == 0 {
		return fmt.Errorf("无可发布的局部细化控制")
	}
	for _, c := range p.Clusters {
		if len(c.CentreMM) != 3 || c.BodyID <= 0 || c.RadiusMM <= 0 || c.SizeMM <= 0 || len(c.ElementIDs) == 0 {
			return fmt.Errorf("无效的局部影响域")
		}
	}
	return nil
}
func validateWarningPlan(p WarningPlan, snapshot *Snapshot, reviewedPlanID string) error {
	if err := validateWarningDiagnostic(p, snapshot, reviewedPlanID); err != nil {
		return err
	}
	if p.Status != "ready" {
		return fmt.Errorf("诊断已完成，但没有可发布的安全细化候选；请查看拒绝原因")
	}
	return nil
}

// SaveWarningPlanHTML renders the EXACT worker-produced proposal. Its numeric
// values are never recomputed/guessed by the UI layer.
func (w Workspace) SaveWarningPlanHTML(s *Snapshot, reviewedPlanID string) (string, error) {
	b, err := os.ReadFile(w.path("warning_refinement_plan.json"))
	if err != nil {
		return "", err
	}
	var p WarningPlan
	if err = json.Unmarshal(b, &p); err != nil {
		return "", err
	}
	if err = validateWarningDiagnostic(p, s, reviewedPlanID); err != nil {
		return "", err
	}
	existingControls, known, controlErr := snapshotWarningControls(s)
	if controlErr != nil {
		existingControls = nil
		known = false
	}
	reportData := warningPlanReportData{WarningPlan: p, ExistingControls: existingControls,
		ExistingControlsKnown: known, ExistingBudget: warningSnapshotBudget(existingControls),
		BudgetView:  computeBudgetDiagnostic(p, existingControls, known),
		FeatureView: diagnosticSmallFeatures(s, p)}
	const page = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>AutoMesh 警告区局部细化预览</title><style>body{font:15px/1.6 "Microsoft YaHei",Arial,sans-serif;color:#172c4b;background:#f5f7fc;margin:0;padding:32px}main{max-width:1150px;margin:auto;background:white;padding:32px;border-radius:16px}h1{margin:0 0 12px}small{color:#64748b}.lead{background:#eef3fb;padding:16px;border-radius:10px}table{border-collapse:collapse;width:100%;margin-top:18px}td,th{padding:11px;border-bottom:1px solid #dde5ef;text-align:left;vertical-align:top;word-break:break-word}th{background:#eef3fb}p{margin:12px 0}code{font-size:12px}strong{font-weight:700}</style></head><body><main><h1>{{if eq .QualityMetric "ElementQuality"}}低 Element Quality 单元分簇优化方案{{else}}高偏度单元分簇优化方案{{end}} · 逐簇审核</h1><small>只读报告，不是 ANSYS 截图；不会生成网格或修改任何 Sizing</small><div class="lead"><p><strong>警告单元</strong> {{.WarningCount}} 个，候选覆盖 {{.CoveredCount}} 个，暂不覆盖 {{.UncoveredCount}} 个；列出 {{len .Clusters}} 处候选影响域；由用户在 AutoMesh 勾选发布</p><p>对应原网格：{{.MeshElements}} 单元。局部体积/尺寸³ 代理量：{{.TotalCellProxy}}（仅为资源代理，不是实际新增单元数）</p><p>工程：<code>{{.Project}}</code></p><p>方案 ID：<code>{{.PlanID}}</code></p></div><table><thead><tr><th>排序编号</th><th>{{if eq $.QualityMetric "ElementQuality"}}实测最低 Element Quality{{else}}实测最大偏度{{end}}</th><th>几何 Body ID / 作用域</th><th>球心 X/Y/Z（mm）</th><th>半径（mm）</th><th>目标尺寸（mm）</th><th>预算代理</th><th>片间安全核验</th><th>覆盖单元 ID</th></tr></thead><tbody>{{range $i, $c := .Clusters}}<tr><td>{{add $i 1}}</td><td>{{if eq $.QualityMetric "ElementQuality"}}{{if $c.EQMin}}{{skewFormat $c.EQMin}}{{else}}未取得{{end}}{{else}}{{if $c.SkewMax}}{{skewFormat $c.SkewMax}}{{else}}未取得{{end}}{{end}}</td><td>{{$c.BodyID}} / {{$c.ScopeKind}}<small>几何来源：{{joinStrings $c.Mapping}}</small></td><td>{{range $j,$v := $c.CentreMM}}{{if $j}}, {{end}}{{printf "%.4f" $v}}{{end}}</td><td>{{printf "%.3f" $c.RadiusMM}}</td><td>{{printf "%.3f" $c.SizeMM}}</td><td>{{$c.Proxy}}</td><td>{{$c.WaferGap}}（需人工确认）</td><td>{{joinIDs $c.ElementIDs}}</td></tr>{{end}}</tbody></table>{{if .ExistingControlsKnown}}{{if .ExistingControls}}<h2>当前模型已有历史 Sizing（保留，不修改）</h2><p>已从当前 ACT 模型树读取 <strong>{{len .ExistingControls}}</strong> 个球形影响域控制；累计预算代理约 <strong>{{.ExistingBudget}}</strong>。这些控制属于先前优化轮次，不要求其旧候选 ID 出现在本轮网格中。新发布前仍会由 ACT 重新读取对象 ID、Body、球心、半径、尺寸和坐标系。</p><table><thead><tr><th>历史 Sizing 名称</th><th>对象 ID</th><th>Body</th><th>球心 X/Y/Z（mm）</th><th>半径（mm）</th><th>尺寸（mm）</th><th>预算代理</th></tr></thead><tbody>{{range .ExistingControls}}<tr><td>{{.Name}}</td><td>{{.ObjectID}}</td><td>{{.BodyID}}</td><td>{{range $i,$v := .CentreMM}}{{if $i}}, {{end}}{{printf "%.4f" $v}}{{end}}</td><td>{{printf "%.3f" .RadiusMM}}</td><td>{{printf "%.3f" .SizeMM}}</td><td>{{.Proxy}}</td></tr>{{end}}</tbody></table>{{else}}<p>当前模型未读取到 AutoMesh_QR 历史 Sizing。</p>{{end}}{{else}}<p>本次只读扫描未提供完整历史 Sizing 清单；本报告仅展示当前候选，不代表历史控制已核验。</p>{{end}}<h2>资源预算与历史覆盖诊断（只读）</h2><p>累计预算建议预警值：120000（代理量，非实际单元数或内存）；历史已用：{{if .BudgetView.Known}}{{.BudgetView.Historical}}{{else}}未核验{{end}}；剩余可用：{{if .BudgetView.Known}}{{.BudgetView.Remaining}}{{else}}未核验{{end}}；候选中最低代理：{{.BudgetView.MinimumCandidate}}。</p><p>{{.BudgetView.Summary}} 超预警需单独人工确认；不删除或修改历史 Sizing、不自动生成网格。</p><table><thead><tr><th>候选</th><th>预算代理</th><th>历史影响球关系</th><th>预算状态</th><th>人工优化建议（非已证实成因）</th></tr></thead><tbody>{{range .BudgetView.Rows}}<tr><td>{{.ID}}</td><td>{{.Proxy}}</td><td>{{.Relation}}</td><td>{{.BudgetStatus}}</td><td>{{.Advice}}</td></tr>{{end}}</tbody></table><h2>小凸起 / 短边 / 狭长面定位与人工细化建议（只读）</h2><p>仅使用当前完整几何快照与已标记单元所对应的 Body、球心。包围盒匹配不是 Face 真实归属证据；短边长度和 CAD Face/Edge ID 仅供在 Meshing 中人工定位，不会自动创建任何 Face/Edge Sizing，也不减少历史预算。为限制计算量，单次报告最多分析前 256 个候选。</p><table><thead><tr><th>候选</th><th>CAD 几何线索</th><th>识别依据与提示</th><th>人工建议</th></tr></thead><tbody>{{range .FeatureView}}<tr><td><code>{{.CandidateID}}</code> / Body {{.BodyID}}</td><td>{{.Summary}}{{range .FaceDetails}}<p>Face <strong>{{.FaceID}}</strong>{{if .EdgeID}}；短边参考 Edge <strong>{{.EdgeID}}</strong>（{{printf "%.3f" .ShortestEdgeMM}} mm）{{end}}</p>{{end}}</td><td>{{range .FaceDetails}}<p>{{.Method}}：{{.Hint}}{{if finiteDist .DistanceMM}}；包围盒距离 {{printf "%.3f" .DistanceMM}} mm{{end}}</p>{{end}}</td><td>{{.Advice}}</td></tr>{{end}}</tbody></table><p><strong>操作边界：</strong>发现候选后，必须先在 Meshing 图形窗口核实该 CAD Face/Edge 与低质量单元的真实关系；同一 Body 的 CAD Face 可人工建 Face Named Selection，随后使用原有局部细化流程；仅 CAD Edge 的建议需在 Meshing 中手动检查 Edge Sizing，当前 AutoMesh 不自动发布 Edge Sizing。未核验的几何线索不得自动发布，也不应为了降低偏度直接删去有物理意义的小凸起。</p><p><strong>注意：</strong>{{.Notes}}</p>{{if eq .Status "no_safe_candidates"}}<p><strong>诊断已完成：没有安全的自动细化候选，不会发布 Sizing，也不修改网格。</strong></p>{{end}}<h2>未覆盖区域及原因</h2><table><thead><tr><th>警告单元 ID / 数量</th><th>排除原因</th><th>候选半径 mm / 预算（如适用）</th></tr></thead><tbody>{{range .RejectedGroups}}<tr><td>{{joinIDs .ElementIDs}}{{if .Count}}（{{.Count}} 单元）{{end}}</td><td>{{reasonZh .Reason}}<small>({{.Reason}})</small></td><td>{{printf "%.3f" .ProposedRadiusMM}} / {{.Proxy}}</td></tr>{{end}}</tbody></table><p>安全审核：每个候选最多 64 个标记单元、影响半径不大于 12 mm；本次列表包含完整分析得到的候选（最多 4096 条）；实际勾选发布每批最多 6 组，已有与新增预算代理超过 120000 时额外预警并要求人工确认；真正的身份、重复球与几何约束保持硬拦截。并核对原有面、边及实体的已生效 Sizing，避免覆盖更细的手工控制。若 ACT 不能真实测量晶圆间净空，本报告会明确标注“需人工确认”，不会冒充几何验证已通过。</p><p>发布仅创建可编辑 Sizing + 坐标系，不会删除现有网格或执行 GenerateMesh。发布后由用户决定是否手动重划；重新检查原标记质量指标、偏度及正交质量，确认有无改善。</p></main></body></html>`
	funcs := template.FuncMap{"add": func(a, b int) int { return a + b },
		"joinStrings": func(items []string) string { return strings.Join(items, ", ") },
		"finiteDist":  featureFinite,
		"skewFormat": func(value *float64) string {
			if value == nil {
				return "未取得"
			}
			return fmt.Sprintf("%.5f", *value)
		},
		"reasonZh": func(s string) string {
			return map[string]string{
				"candidate_count_limit":            "候选过多需继续分批审核，不会自动截断为 24 个",
				"influence_radius_exceeds_limit":   "单簇影响半径超过 12 mm",
				"already_has_finer_control":        "已有更细 Sizing，细化到允许下限仍可能冲突",
				"estimated_cell_budget_exceeded":   "预计局部体积/尺寸预算超限",
				"ambiguous_geometry_body":          "无法唯一绑定几何 Body",
				"inconsistent_node_body_ownership": "节点与几何 Body 映射冲突",
				"element_geometry_unreadable":      "单元坐标或拓扑无法安全读取",
				"analysis_element_count_limit":     "超过单次 ACT 安全分析上限，请分批筛选",
				"invalid_element_span":             "单元尺寸异常，建议核查局部几何",
			}[s]
		},
		"joinIDs": func(ids []int) string {
			ss := make([]string, len(ids))
			for i, v := range ids {
				ss[i] = fmt.Sprint(v)
			}
			return strings.Join(ss, ", ")
		}}
	templ, err := template.New("plan").Funcs(funcs).Parse(page)
	if err != nil {
		return "", err
	}
	path := filepath.Join(w.Dir, "warning_refinement_plan.html")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	e := templ.Execute(f, reportData)
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	return path, nil
}
