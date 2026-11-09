package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// WarningSelection is a deliberate, per-candidate review receipt. It is not
// generated automatically when a plan is previewed; no checked boxes => no
// published Sizing. The Meshing worker independently revalidates every field.
type WarningSelection struct {
	SchemaVersion                int      `json:"schema_version"`
	PlanID                       string   `json:"plan_id"`
	Project                      string   `json:"project"`
	Fingerprint                  string   `json:"geometry_fingerprint"`
	MeshElements                 int      `json:"mesh_elements"`
	CandidateIDs                 []string `json:"candidate_ids"`
	ManuallyReviewedCandidateIDs []string `json:"manually_reviewed_candidate_ids"`
	SelectedBudget               int      `json:"selected_budget_proxy"`
	BudgetWarningAcknowledged    bool     `json:"budget_warning_acknowledged,omitempty"`
	CreatedAt                    string   `json:"created_at"`
}

type warningOwnedManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Project       string `json:"project"`
	Fingerprint   string `json:"fingerprint"`
	Controls      []struct {
		CandidateID      string    `json:"candidate_id"`
		Name             string    `json:"name"`
		ObjectID         int       `json:"object_id"`
		CoordinateSystem string    `json:"coordinate_system"`
		CSObjectID       int       `json:"cs_object_id"`
		BodyID           int       `json:"body_id"`
		CentreMM         []float64 `json:"centre_mm"`
		RadiusMM         float64   `json:"radius_mm"`
		SizeMM           float64   `json:"size_mm"`
		Proxy            int       `json:"cell_budget_proxy"`
	} `json:"controls"`
}

type warningControlSnapshot struct {
	Name             string
	ObjectID         int
	CoordinateSystem string
	CSObjectID       int
	BodyID           int
	CentreMM         []float64
	RadiusMM         float64
	SizeMM           float64
	Proxy            int
	Suppressed       bool
}

// liveWarningSizingControls reports whether the current read-only snapshot
// contains reserved AutoMesh quality-hotspot Sizing controls. The snapshot's
// Controls list is produced directly from Model.Mesh.Children during inspect.
// A nil list is treated as unknown, never as proof that old controls are gone.
func liveWarningSizingControls(snapshot *Snapshot) (found, known bool) {
	if snapshot == nil || snapshot.Controls == nil {
		return false, false
	}
	for _, control := range snapshot.Controls {
		name, ok := control["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return false, false
		}
		if strings.HasPrefix(name, "AutoMesh_QR_") {
			return true, true
		}
	}
	return false, true
}

func numericSnapshotValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		x, err := n.Float64()
		return x, err == nil
	default:
		return 0, false
	}
}

func snapshotIntSlice(v any) ([]int, bool) {
	items, ok := v.([]any)
	if !ok {
		if ints, ok2 := v.([]int); ok2 {
			return append([]int(nil), ints...), true
		}
		return nil, false
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		n, ok := numericSnapshotValue(item)
		if !ok || math.Trunc(n) != n {
			return nil, false
		}
		out = append(out, int(n))
	}
	return out, true
}

func snapshotFloatSlice(v any) ([]float64, bool) {
	items, ok := v.([]any)
	if !ok {
		if values, ok2 := v.([]float64); ok2 {
			return append([]float64(nil), values...), true
		}
		return nil, false
	}
	out := make([]float64, 0, len(items))
	for _, item := range items {
		n, ok := numericSnapshotValue(item)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// snapshotWarningControls parses only the reserved live quality-hotspot
// controls. It requires a complete read-only ACT inventory and exact paired
// coordinate-system records before those controls can unlock incremental
// publication from a stale/missing manifest.
func snapshotWarningControls(snapshot *Snapshot) ([]warningControlSnapshot, bool, error) {
	if snapshot == nil || snapshot.Controls == nil {
		return nil, false, nil
	}
	var raw []map[string]any
	for _, item := range snapshot.Controls {
		name, ok := item["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, false, errors.New("当前 Meshing 控件清单存在无法识别的对象名称")
		}
		if strings.HasPrefix(name, "AutoMesh_QR_") {
			raw = append(raw, item)
		}
	}
	if len(raw) == 0 {
		if len(snapshot.WarningCoordinateSystems) > 0 {
			return nil, false, errors.New("发现未配对的 AutoMesh_QR 球心坐标系")
		}
		return []warningControlSnapshot{}, true, nil
	}
	if !snapshot.ControlsComplete || !snapshot.WarningCoordinateSystemsComplete {
		return nil, false, errors.New("当前扫描未提供完整的 Sizing/坐标系清单，无法接管历史控制")
	}
	bodySet := map[int]bool{}
	for _, body := range snapshot.Bodies {
		bodySet[body.ID] = true
	}
	csSet := map[string]int{}
	for _, item := range snapshot.WarningCoordinateSystems {
		name, _ := item["name"].(string)
		idf, ok := numericSnapshotValue(item["object_id"])
		if !ok || name == "" || !strings.HasPrefix(name, "AutoMesh_QR_CS_") {
			return nil, false, errors.New("历史球心坐标系身份不完整")
		}
		key := fmt.Sprintf("%s#%d", name, int(idf))
		if _, exists := csSet[key]; exists {
			return nil, false, errors.New("历史球心坐标系存在重复身份")
		}
		csSet[key] = int(idf)
	}
	controls := make([]warningControlSnapshot, 0, len(raw))
	seenNames, seenIDs, usedCS := map[string]bool{}, map[int]bool{}, map[string]bool{}
	for _, item := range raw {
		name := item["name"].(string)
		if errText, _ := item["warning_control_error"].(string); errText != "" {
			return nil, false, fmt.Errorf("历史 Sizing %s 无法完整读取：%s", name, errText)
		}
		category, _ := item["category"].(string)
		isSizing, _ := item["is_sizing"].(bool)
		typ, _ := item["type"].(string)
		if !isSizing && !strings.Contains(strings.ToLower(category), "sizing") {
			return nil, false, fmt.Errorf("保留名称 %s 对应对象不是 Sizing", name)
		}
		if !strings.Contains(strings.ToLower(typ), "sphereofinfluence") {
			return nil, false, fmt.Errorf("历史 Sizing %s 不是球形影响域控制", name)
		}
		objf, ok := numericSnapshotValue(item["object_id"])
		if !ok || objf <= 0 {
			return nil, false, fmt.Errorf("历史 Sizing %s 缺少对象 ID", name)
		}
		objectID := int(objf)
		if seenNames[name] || seenIDs[objectID] {
			return nil, false, errors.New("历史 Sizing 存在重复名称或对象 ID")
		}
		seenNames[name], seenIDs[objectID] = true, true
		bodies, ok := snapshotIntSlice(item["body_ids"])
		if !ok || len(bodies) != 1 || !bodySet[bodies[0]] {
			return nil, false, fmt.Errorf("历史 Sizing %s 的 Body 作用域无法唯一核验", name)
		}
		size, ok1 := numericSnapshotValue(item["size_mm"])
		radius, ok2 := numericSnapshotValue(item["radius_mm"])
		centre, ok3 := snapshotFloatSlice(item["centre_mm"])
		if !ok1 || !ok2 || !ok3 || len(centre) != 3 || size < 0.5 || size > 5 || radius <= 0 || radius > 12 {
			return nil, false, fmt.Errorf("历史 Sizing %s 的尺寸/半径/球心不完整", name)
		}
		csName, _ := item["coordinate_system_name"].(string)
		csIDf, ok := numericSnapshotValue(item["coordinate_system_object_id"])
		if !ok || csName == "" || !strings.HasPrefix(csName, "AutoMesh_QR_CS_") {
			return nil, false, fmt.Errorf("历史 Sizing %s 的球心坐标系无法读取", name)
		}
		csKey := fmt.Sprintf("%s#%d", csName, int(csIDf))
		if _, exists := csSet[csKey]; !exists || usedCS[csKey] {
			return nil, false, fmt.Errorf("历史 Sizing %s 的球心坐标系未能唯一匹配", name)
		}
		usedCS[csKey] = true
		if csName != "AutoMesh_QR_CS_"+strings.TrimPrefix(name, "AutoMesh_QR_") {
			return nil, false, fmt.Errorf("历史 Sizing %s 与球心坐标系命名不匹配", name)
		}
		suppressed, _ := item["suppressed"].(bool)
		if suppressed {
			return nil, false, fmt.Errorf("历史 Sizing %s 已停用，需人工核对", name)
		}
		proxy := int(math.Ceil((4.0 / 3.0) * math.Pi * math.Pow(radius/size, 3)))
		controls = append(controls, warningControlSnapshot{Name: name, ObjectID: objectID, CoordinateSystem: csName, CSObjectID: int(csIDf), BodyID: bodies[0], CentreMM: centre, RadiusMM: radius, SizeMM: size, Proxy: proxy, Suppressed: suppressed})
	}
	if len(usedCS) != len(csSet) {
		return nil, false, errors.New("存在未被历史 Sizing 使用的 AutoMesh_QR 球心坐标系")
	}
	return controls, true, nil
}

func warningSnapshotBudget(controls []warningControlSnapshot) int {
	total := 0
	for _, control := range controls {
		total += control.Proxy
	}
	return total
}

func candidateMatchesSnapshotControl(c WarningHotspot, control warningControlSnapshot) bool {
	return c.BodyID == control.BodyID && candidateMatchesOwned(c, control.BodyID, control.CentreMM, control.RadiusMM, control.SizeMM)
}

func markSnapshotMatchedWarningCandidates(snapshot *Snapshot, plan WarningPlan, published map[string]bool) error {
	controls, known, err := snapshotWarningControls(snapshot)
	if err != nil {
		return err
	}
	if !known {
		return errors.New("当前历史 Sizing 清单未能完整读取")
	}
	for _, control := range controls {
		matches := []string{}
		for _, candidate := range plan.Clusters {
			if candidateMatchesSnapshotControl(candidate, control) {
				matches = append(matches, candidate.ID)
			}
		}
		if len(matches) == 1 {
			published[matches[0]] = true
		}
	}
	return nil
}

// A shared Data directory can retain the last warning-Sizing manifest from a
// different model. It is safe to ignore that archive only when the current
// inspect snapshot positively proves that no AutoMesh_QR_* Sizing is live.
// This mirrors the ACT worker's _current_warning_owner() rule.
func staleWarningManifestDisposition(snapshot *Snapshot) error {
	found, known := liveWarningSizingControls(snapshot)
	if known && !found {
		return nil
	}
	if found {
		return errors.New("历史 Sizing 发布记录属于不同工程/几何，当前 Meshing 仍存在 AutoMesh_QR_* 控件；为避免重复或误判资源预算，当前方案仅允许查看，禁止发布。请先人工核对旧控制")
	}
	return errors.New("历史 Sizing 发布记录属于不同工程/几何，且当前扫描未提供完整控制清单；无法证明旧控制已不存在。当前方案仅允许查看，禁止发布")
}

func (w Workspace) warningOwnedCandidates(snapshot *Snapshot) (map[string]bool, int, error) {
	owned := map[string]bool{}
	m, err := readJSON[warningOwnedManifest](w.path("warning_refinement_managed.json"))
	if err != nil {
		controls, known, parseErr := snapshotWarningControls(snapshot)
		if parseErr != nil {
			return nil, 0, parseErr
		}
		if known {
			// Missing/corrupt shared metadata can be rebuilt from the current ACT
			// model only when every live QR control is fully read back.
			return owned, warningSnapshotBudget(controls), nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return owned, 0, nil
		}
		return nil, 0, fmt.Errorf("已有警告区发布记录无法读取，且当前 ACT 控件清单不完整：%w", err)
	}
	if len(m.Controls) > 0 && (snapshot == nil || m.Project != snapshot.Project || m.Fingerprint != snapshot.Fingerprint) {
		controls, known, parseErr := snapshotWarningControls(snapshot)
		if parseErr == nil && known && len(controls) == 0 {
			return owned, 0, nil
		}
		if parseErr == nil && known {
			return owned, warningSnapshotBudget(controls), nil
		}
		if parseErr != nil {
			return nil, 0, parseErr
		}
		if dispositionErr := staleWarningManifestDisposition(snapshot); dispositionErr != nil {
			return nil, 0, dispositionErr
		}
		return owned, 0, nil
	}
	controls, known, parseErr := snapshotWarningControls(snapshot)
	if parseErr != nil {
		return nil, 0, parseErr
	}
	if known && len(m.Controls) == 0 && len(controls) > 0 {
		return owned, warningSnapshotBudget(controls), nil
	}
	if known && len(m.Controls) > 0 {
		if len(controls) != len(m.Controls) {
			return nil, 0, errors.New("当前 Meshing 的 AutoMesh_QR 控件数量与发布记录不一致，需人工核对")
		}
		byKey := map[string]warningControlSnapshot{}
		for _, control := range controls {
			byKey[fmt.Sprintf("%s#%d", control.Name, control.ObjectID)] = control
		}
		for _, entry := range m.Controls {
			if entry.Name == "" || entry.ObjectID <= 0 {
				return nil, 0, errors.New("历史 Sizing 发布记录缺少对象身份，无法核对")
			}
			actual, ok := byKey[fmt.Sprintf("%s#%d", entry.Name, entry.ObjectID)]
			if !ok || !candidateMatchesSnapshotControl(WarningHotspot{BodyID: entry.BodyID, CentreMM: entry.CentreMM, RadiusMM: entry.RadiusMM, SizeMM: entry.SizeMM}, actual) {
				return nil, 0, fmt.Errorf("历史 Sizing %s 与当前 ACT 控件不一致，可能被人工修改", entry.Name)
			}
			if entry.CandidateID != "" {
				owned[entry.CandidateID] = true
			}
		}
		return owned, warningSnapshotBudget(controls), nil
	}
	budget := 0
	for _, c := range m.Controls {
		if c.CandidateID != "" {
			owned[c.CandidateID] = true
		}
		b := c.Proxy
		if b <= 0 && c.RadiusMM > 0 && c.SizeMM > 0 {
			b = int(math.Ceil((4 * math.Pi / 3) * math.Pow(c.RadiusMM/c.SizeMM, 3)))
		}
		if b <= 0 {
			return nil, 0, errors.New("历史警告区 Sizing 预算不可核验，请人工核对")
		}
		budget += b
	}
	return owned, budget, nil
}

// validateWarningSelection runs before any task submission or ACT changes.
func validateWarningSelection(p WarningPlan, ids []string, published map[string]bool, existingBudget int) (WarningSelection, error) {
	return validateWarningSelectionWithBudgetAck(p, ids, published, existingBudget, false)
}

// The budget proxy is only an advisory threshold. Crossing it requires a
// deliberate operator acknowledgement, recorded in the task-bound receipt.
func validateWarningSelectionWithBudgetAck(p WarningPlan, ids []string, published map[string]bool, existingBudget int, budgetWarningAck bool) (WarningSelection, error) {
	s := WarningSelection{}
	if p.Status != "ready" || len(p.Clusters) == 0 || p.PlanID == "" {
		return s, errors.New("没有有效的候选方案")
	}
	if len(ids) == 0 {
		return s, errors.New("请至少勾选一个未发布的候选区域")
	}
	if len(ids) > 4096 {
		return s, errors.New("所选区域数超过安全分析上限")
	}
	index := make(map[string]WarningHotspot, len(p.Clusters))
	for _, c := range p.Clusters {
		if c.ID == "" || len(c.ID) != 20 {
			return s, errors.New("旧版方案没有稳定候选 ID，请重新生成局部方案")
		}
		if _, ok := index[c.ID]; ok {
			return s, errors.New("方案存在重复候选 ID")
		}
		index[c.ID] = c
	}
	used := map[string]bool{}
	budget := existingBudget
	for _, id := range ids {
		c, ok := index[id]
		if !ok || used[id] {
			return s, fmt.Errorf("未知或重复的候选：%s", id)
		}
		if published[id] {
			return s, fmt.Errorf("候选 %s 已发布，禁止重复创建 Sizing", id)
		}
		if c.BodyID <= 0 || c.RadiusMM <= 0 || c.RadiusMM > 12 || c.SizeMM < 0.5 || c.SizeMM > 5 || len(c.ElementIDs) == 0 || c.Proxy <= 0 {
			return s, fmt.Errorf("候选 %s 无法通过局部安全检查", id)
		}
		if c.WaferGap != "manual_review_required" && c.WaferGap != "verified" {
			return s, fmt.Errorf("候选 %s 片间风险状态不可识别", id)
		}
		budget += c.Proxy
		used[id] = true
	}
	if _, err := reviewedSphereOverlap(p, ids, published); err != nil {
		return s, err
	}
	if budget > reviewBudgetLimit && !budgetWarningAck {
		return s, fmt.Errorf("累计预算代理 %d 超过预警值 %d，需要独立人工确认", budget, reviewBudgetLimit)
	}
	return WarningSelection{SchemaVersion: 1, PlanID: p.PlanID, Project: p.Project,
		Fingerprint: p.Fingerprint, MeshElements: p.MeshElements,
		CandidateIDs:                 append([]string(nil), ids...),
		ManuallyReviewedCandidateIDs: append([]string(nil), ids...),
		SelectedBudget:               budget, BudgetWarningAcknowledged: budget > reviewBudgetLimit && budgetWarningAck, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (w Workspace) saveWarningSelection(snapshot *Snapshot, p WarningPlan, ids []string) (WarningSelection, error) {
	return w.saveWarningSelectionWithBudgetAck(snapshot, p, ids, false)
}

func (w Workspace) saveWarningSelectionWithBudgetAck(snapshot *Snapshot, p WarningPlan, ids []string, budgetWarningAck bool) (WarningSelection, error) {
	if snapshot == nil || p.Project != snapshot.Project || p.Fingerprint != snapshot.Fingerprint {
		return WarningSelection{}, errors.New("候选方案与当前工程不符")
	}
	owned, budget, err := w.warningOwnedCandidates(snapshot)
	if err != nil {
		return WarningSelection{}, err
	}
	selection, err := validateWarningSelectionWithBudgetAck(p, ids, owned, budget, budgetWarningAck)
	if err != nil {
		return WarningSelection{}, err
	}
	if err := writeJSON(w.path("warning_refinement_selection.json"), selection); err != nil {
		return WarningSelection{}, err
	}
	return selection, nil
}

func (w Workspace) readWarningSelection(p WarningPlan) (WarningSelection, error) {
	s, err := readJSON[WarningSelection](w.path("warning_refinement_selection.json"))
	if err != nil {
		return s, errors.New("尚未确认勾选候选，请打开审核列表重新选择")
	}
	if s.SchemaVersion != 1 || s.PlanID != p.PlanID || s.Project != p.Project || s.Fingerprint != p.Fingerprint || s.MeshElements != p.MeshElements || len(s.CandidateIDs) == 0 {
		return s, errors.New("候选勾选与当前已审核方案不一致，禁止发布")
	}
	if s.SelectedBudget > reviewBudgetLimit && !s.BudgetWarningAcknowledged {
		return s, errors.New("超建议预算的发布记录缺少人工风险确认")
	}
	if len(s.CandidateIDs) != len(s.ManuallyReviewedCandidateIDs) {
		return s, errors.New("部分候选尚未确认安全风险")
	}
	ids := append([]string(nil), s.CandidateIDs...)
	acks := append([]string(nil), s.ManuallyReviewedCandidateIDs...)
	sort.Strings(ids)
	sort.Strings(acks)
	if strings.Join(ids, "|") != strings.Join(acks, "|") {
		return s, errors.New("所选候选的人工核查未完成")
	}
	return s, nil
}

// Mark unambiguous legacy candidates as already published for display only.
// The worker verifies the ACT controls themselves before any new write.
func (w Workspace) reconcileReviewLegacyPublished(snapshot *Snapshot, plan WarningPlan, published map[string]bool) error {
	m, err := readJSON[warningOwnedManifest](w.path("warning_refinement_managed.json"))
	if errors.Is(err, os.ErrNotExist) {
		return markSnapshotMatchedWarningCandidates(snapshot, plan, published)
	}
	if err != nil {
		if _, known, parseErr := snapshotWarningControls(snapshot); parseErr == nil && known {
			return markSnapshotMatchedWarningCandidates(snapshot, plan, published)
		} else if parseErr != nil {
			return parseErr
		}
		return fmt.Errorf("旧版警告区控制记录无法读取，且当前 ACT 控件清单不完整：%w", err)
	}
	if len(m.Controls) > 0 && (snapshot == nil || m.Project != snapshot.Project || m.Fingerprint != snapshot.Fingerprint) {
		controls, known, parseErr := snapshotWarningControls(snapshot)
		if parseErr == nil && known && len(controls) == 0 {
			return nil
		}
		if parseErr != nil {
			return parseErr
		}
		if !known {
			return staleWarningManifestDisposition(snapshot)
		}
		return markSnapshotMatchedWarningCandidates(snapshot, plan, published)
	}
	for _, c := range m.Controls {
		if c.CandidateID != "" {
			var matching *WarningHotspot
			for i := range plan.Clusters {
				if plan.Clusters[i].ID == c.CandidateID {
					matching = &plan.Clusters[i]
					break
				}
			}
			if matching == nil {
				// Expected after a remesh: historical candidate IDs contain old
				// element IDs. Keep the live Sizing as history; do not block new IDs.
				continue
			}
			if !candidateMatchesOwned(*matching, c.BodyID, c.CentreMM, c.RadiusMM, c.SizeMM) {
				return errors.New("同一候选 ID 的 Sizing 球心/半径/尺寸与当前方案不同；拒绝重复发布")
			}
			published[c.CandidateID] = true
			continue
		}
		var matches []string
		for _, x := range plan.Clusters {
			if x.BodyID == c.BodyID && candidateMatchesOwned(x, c.BodyID, c.CentreMM, c.RadiusMM, c.SizeMM) {
				matches = append(matches, x.ID)
			}
		}
		if len(matches) == 1 {
			published[matches[0]] = true
		}
	}
	return nil
}

func candidateMatchesOwned(c WarningHotspot, body int, centre []float64, radius, size float64) bool {
	if c.BodyID != body || len(c.CentreMM) != 3 || len(centre) != 3 {
		return false
	}
	if math.Abs(c.RadiusMM-radius) > 0.001 || math.Abs(c.SizeMM-size) > 0.001 {
		return false
	}
	for i := 0; i < 3; i++ {
		if math.Abs(c.CentreMM[i]-centre[i]) > 0.001 {
			return false
		}
	}
	return true
}

func candidateCentersDistance(a, b WarningHotspot) float64 {
	if len(a.CentreMM) != 3 || len(b.CentreMM) != 3 {
		return math.Inf(1)
	}
	var sq float64
	for i := 0; i < 3; i++ {
		d := a.CentreMM[i] - b.CentreMM[i]
		sq += d * d
	}
	return math.Sqrt(sq)
}

// Inform the user about actual sphere overlap; near-identical same-body
// SOIs are blocked, while ordinary intersections require manual visual review.
func reviewedSphereOverlap(p WarningPlan, ids []string, published map[string]bool) (int, error) {
	index := make(map[string]WarningHotspot, len(p.Clusters))
	for _, c := range p.Clusters {
		index[c.ID] = c
	}
	items := make([]WarningHotspot, 0, len(ids)+len(published))
	for _, id := range ids {
		items = append(items, index[id])
	}
	for id := range published {
		if c, ok := index[id]; ok {
			items = append(items, c)
		}
	}
	overlaps := 0
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			a, b := items[i], items[j]
			if a.BodyID != b.BodyID {
				continue
			}
			d := candidateCentersDistance(a, b)
			if d < 0.25*math.Min(a.RadiusMM, b.RadiusMM) {
				return 0, fmt.Errorf("候选 %s 和 %s 球心几乎重合，可能重复细化；请重新选择", a.ID, b.ID)
			}
			if d < a.RadiusMM+b.RadiusMM {
				overlaps++
			}
		}
	}
	return overlaps, nil
}
