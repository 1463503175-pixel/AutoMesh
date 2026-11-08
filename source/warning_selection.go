package main

import (
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
	CreatedAt                    string   `json:"created_at"`
}

type warningOwnedManifest struct {
	Project     string `json:"project"`
	Fingerprint string `json:"fingerprint"`
	Controls    []struct {
		CandidateID string    `json:"candidate_id"`
		BodyID      int       `json:"body_id"`
		CentreMM    []float64 `json:"centre_mm"`
		RadiusMM    float64   `json:"radius_mm"`
		SizeMM      float64   `json:"size_mm"`
		Proxy       int       `json:"cell_budget_proxy"`
	} `json:"controls"`
}

func (w Workspace) warningOwnedCandidates(snapshot *Snapshot) (map[string]bool, int, error) {
	owned := map[string]bool{}
	m, err := readJSON[warningOwnedManifest](w.path("warning_refinement_managed.json"))
	if err != nil {
		// No prior published manifest is normal. Malformed manifest is NOT.
		if errors.Is(err, os.ErrNotExist) {
			return owned, 0, nil
		}
		return nil, 0, fmt.Errorf("已有警告区发布记录无法读取：%w", err)
	}
	if len(m.Controls) > 0 && (snapshot == nil || m.Project != snapshot.Project || m.Fingerprint != snapshot.Fingerprint) {
		return nil, 0, errors.New("历史 Sizing 发布记录属于不同工程/几何，请先核对旧控制")
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
	if budget > 120000 {
		return s, fmt.Errorf("累计预算 %d > 120000；请减少本次勾选，未选候选会保留", budget)
	}
	return WarningSelection{SchemaVersion: 1, PlanID: p.PlanID, Project: p.Project,
		Fingerprint: p.Fingerprint, MeshElements: p.MeshElements,
		CandidateIDs:                 append([]string(nil), ids...),
		ManuallyReviewedCandidateIDs: append([]string(nil), ids...),
		SelectedBudget:               budget, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (w Workspace) saveWarningSelection(snapshot *Snapshot, p WarningPlan, ids []string) (WarningSelection, error) {
	if snapshot == nil || p.Project != snapshot.Project || p.Fingerprint != snapshot.Fingerprint {
		return WarningSelection{}, errors.New("候选方案与当前工程不符")
	}
	owned, budget, err := w.warningOwnedCandidates(snapshot)
	if err != nil {
		return WarningSelection{}, err
	}
	selection, err := validateWarningSelection(p, ids, owned, budget)
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
		return nil
	}
	if err != nil {
		return fmt.Errorf("旧版警告区控制记录无法读取：%w", err)
	}
	if len(m.Controls) > 0 && (snapshot == nil || m.Project != snapshot.Project || m.Fingerprint != snapshot.Fingerprint) {
		return errors.New("历史警告区控制属于其他工程/几何，禁止继续叠加")
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
			if matching == nil || !candidateMatchesOwned(*matching, c.BodyID, c.CentreMM, c.RadiusMM, c.SizeMM) {
				return errors.New("已有 Sizing 的球心/半径/尺寸与当前候选不同。网格可能变化，拒绝使用旧候选直接追加；请人工核查")
			}
			continue
		}
		var matches []string
		for _, x := range plan.Clusters {
			if x.BodyID != c.BodyID || len(c.CentreMM) != 3 {
				continue
			}
			if math.Abs(x.RadiusMM-c.RadiusMM) > 0.001 || math.Abs(x.SizeMM-c.SizeMM) > 0.001 {
				continue
			}
			if math.Abs(x.CentreMM[0]-c.CentreMM[0]) > 0.001 || math.Abs(x.CentreMM[1]-c.CentreMM[1]) > 0.001 || math.Abs(x.CentreMM[2]-c.CentreMM[2]) > 0.001 {
				continue
			}
			matches = append(matches, x.ID)
		}
		if len(matches) != 1 || published[matches[0]] {
			return errors.New("发现 R7.9 自建 Sizing 未能与当前候选唯一匹配。不会覆盖或重复创建；请在 Meshing 中先核对旧控制。")
		}
		published[matches[0]] = true
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
