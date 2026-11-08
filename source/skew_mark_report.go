package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

// SkewMarker is a snapshot of the last *completed* ACT-created element group.
// It is only a read-only summary; publication always revalidates the live NS.
type SkewMarker struct {
	Status         string   `json:"status"`
	Project        string   `json:"project"`
	Fingerprint    string   `json:"fingerprint"`
	MeshCount      int      `json:"mesh_count"`
	WorksheetCount int      `json:"worksheet_count"`
	Source         string   `json:"selection_source"`
	RangeMin       *float64 `json:"range_min"`
	RangeMax       *float64 `json:"range_max"`
	Groups         []struct {
		Name     string `json:"name"`
		ObjectID int    `json:"object_id"`
		IDs      []int  `json:"ids"`
	} `json:"groups"`
}

func (w Workspace) SaveSkewMarkerHTML(s *Snapshot) (string, error) {
	if s == nil || s.Project == "" || s.Fingerprint == "" {
		return "", fmt.Errorf("没有有效的当前工程扫描")
	}
	raw, err := os.ReadFile(w.path("warning_refinement_marker.json"))
	if err != nil {
		return "", err
	}
	var m SkewMarker
	if err = json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	if m.Status != "valid" || m.Source != "named_selection_worksheet_skewness_range" || len(m.Groups) == 0 || m.RangeMin == nil || m.RangeMax == nil {
		return "", fmt.Errorf("当前没有成功创建的自定义偏度范围标记")
	}
	if m.Project != s.Project || m.Fingerprint != s.Fingerprint {
		return "", fmt.Errorf("偏度标记对应的工程或几何已变化")
	}
	if s.Mesh == nil {
		return "", fmt.Errorf("缺少当前网格实测单元数，不能验证偏度标记")
	}
	n, countErr := meshCellCount(s.Mesh["Elements"])
	if countErr != nil || n <= 0 || n != m.MeshCount {
		return "", fmt.Errorf("当前扫描网格与标记时单元数不一致或不可读取，不能展示为当前标记")
	}
	if m.WorksheetCount <= 0 || m.RangeMin == nil || m.RangeMax == nil || *m.RangeMin < 0 || *m.RangeMax > 1 || *m.RangeMin >= *m.RangeMax {
		return "", fmt.Errorf("偏度范围标记数据无效")
	}
	sum := 0
	for _, g := range m.Groups {
		if g.Name == "" || g.ObjectID <= 0 {
			return "", fmt.Errorf("标记组身份无效")
		}
		sum += len(g.IDs)
	}
	if sum != m.WorksheetCount {
		return "", fmt.Errorf("标记数量不一致")
	}
	const html = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>AutoMesh 偏度区间标记</title><style>body{font-family:"Microsoft YaHei",Arial,sans-serif;background:#f5f7fb;color:#173252;padding:32px}main{background:white;max-width:1020px;margin:auto;padding:30px;border-radius:12px}table{border-collapse:collapse;width:100%}td,th{border-bottom:1px solid #ddd;padding:10px;text-align:left}small{color:#5f7188}code{word-break:break-all}</style></head><body><main><h1>偏度单元标记结果（只读）</h1><p>筛选范围：<b>{{fmtFloat .RangeMin}} ～ {{fmtFloat .RangeMax}}</b>；ACT 返回共 <b>{{.WorksheetCount}}</b> 个 Mesh Element 单元；来源：原生 Named Selection Worksheet。</p><p>标记时网格单元数：{{.MeshCount}}；工程：<code>{{.Project}}</code></p><p><b>这只是上次成功标记的结果，并非实时网格复核。</b>生成方案和发布前会重新核对标记组对象及作用域，网格变化后必须重新标记。</p><table><tr><th>元素命名组</th><th>Object ID</th><th>元素数</th><th>部分单元 ID（最多 48 个）</th></tr>{{range .Groups}}<tr><td>{{.Name}}</td><td>{{.ObjectID}}</td><td>{{len .IDs}}</td><td>{{sample .IDs}}</td></tr>{{end}}</table><p><small>此报告不创建 Sizing、不清除网格、不触发 Fluent。</small></p></main></body></html>`
	t, err := template.New("marker").Funcs(template.FuncMap{"fmtFloat": func(v *float64) string { return fmt.Sprintf("%.3f", *v) }, "sample": func(ids []int) string {
		max := len(ids)
		if max > 48 {
			max = 48
		}
		return fmt.Sprint(ids[:max])
	}}).Parse(html)
	if err != nil {
		return "", err
	}
	path := filepath.Join(w.Dir, "skew_range_marker.html")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	err = t.Execute(f, m)
	cerr := f.Close()
	if err != nil {
		return "", err
	}
	if cerr != nil {
		return "", cerr
	}
	return path, nil
}
