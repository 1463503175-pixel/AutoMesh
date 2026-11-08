//go:build windows

package main

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	ICC_LISTVIEW_CLASSES         = 0x00000001
	LVS_REPORT                   = 0x0001
	LVS_SHOWSELALWAYS            = 0x0008
	LVS_EX_CHECKBOXES            = 0x00000004
	LVS_EX_FULLROWSELECT         = 0x00000020
	LVS_EX_DOUBLEBUFFER          = 0x00010000
	LVM_FIRST                    = 0x1000
	LVM_DELETEALLITEMS           = LVM_FIRST + 9
	LVM_SETITEMSTATE             = LVM_FIRST + 43
	LVM_GETITEMSTATE             = LVM_FIRST + 44
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVM_INSERTITEMW              = LVM_FIRST + 77
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_SETITEMTEXTW             = LVM_FIRST + 116
	LVIF_TEXT                    = 1
	LVIS_STATEIMAGEMASK          = 0xF000
	LVCF_WIDTH                   = 2
	LVCF_TEXT                    = 4
	LVCF_SUBITEM                 = 8
	ID_REVIEW_CONFIRM            = 143
	ID_REVIEW_BACK               = 144
	ID_REVIEW_CLEAR              = 145
	ID_REVIEW_DETAILS            = 146
	WM_NOTIFY                    = 0x004E
	LVN_ITEMCHANGED              = ^uint32(100) // LVN_FIRST(-100)-1
)

type initCommonControls struct{ Size, Classes uint32 }
type lvColumn struct {
	Mask         uint32
	Format       int32
	Width        int32
	Text         uintptr
	TextMax      int32
	SubItem      int32
	Image        int32
	Order        int32
	MinWidth     int32
	DefaultWidth int32
	IdealWidth   int32
}
type lvItem struct {
	Mask          uint32
	Item          int32
	SubItem       int32
	State         uint32
	StateMask     uint32
	Text          uintptr
	TextMax       int32
	Image         int32
	Param         uintptr
	Indent        int32
	GroupID       int32
	Columns       uint32
	ColumnIndexes uintptr
	ColumnFormats uintptr
	Group         uintptr
}
type nmHeader struct {
	Window uintptr
	ID     uintptr
	Code   uint32
}

func (a *state) initializeCandidateList() {
	cc := initCommonControls{Size: uint32(unsafe.Sizeof(initCommonControls{})), Classes: ICC_LISTVIEW_CLASSES}
	comctl := syscallNewLazyDLL("comctl32.dll")
	comctl.NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&cc)))
	h := control("SysListView32", "", WS_TABSTOP|WS_BORDER|LVS_REPORT|LVS_SHOWSELALWAYS, 250, 250, 800, 360, 0)
	a.put("reviewlist", h)
	if h == 0 {
		return
	}
	send(h, LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_CHECKBOXES|LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER, LVS_EX_CHECKBOXES|LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER)
	cols := []struct {
		Label string
		Width int
	}{
		{"区域", 108}, {"最大偏度", 102}, {"目标单元", 80}, {"Body", 88}, {"球半径(mm)", 109}, {"尺寸(mm)", 88}, {"预算代理", 99}, {"状态", 155},
	}
	for i, c := range cols {
		item := lvColumn{Mask: LVCF_TEXT | LVCF_WIDTH | LVCF_SUBITEM, Width: int32(c.Width), Text: ptr(c.Label), SubItem: int32(i)}
		send(h, LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&item)))
	}
}

// syscall is in main_windows.go too, but imports are file scoped.
func syscallNewLazyDLL(name string) *syscall.LazyDLL { return syscall.NewLazyDLL(name) }

func (a *state) populateCandidateList() {
	h := a.ctl("reviewlist")
	if h == 0 {
		return
	}
	a.reviewPopulating = true
	defer func() { a.reviewPopulating = false }()
	send(h, LVM_DELETEALLITEMS, 0, 0)
	for i, c := range a.reviewPlan.Clusters {
		quality := "未取得"
		if c.SkewMax != nil {
			quality = fmt.Sprintf("%.5f", *c.SkewMax)
		}
		state := "待人工审核"
		if a.reviewPublishBlockedReason != "" {
			state = "历史待核对·不可发布"
		} else if a.reviewPublished[c.ID] {
			state = "已发布·不可重复"
		} else if near, overlap := a.existingSphereRelation(c); near {
			state = "与历史 Sizing 近重合·不可发布"
		} else if overlap {
			state = "与历史 Sizing 相交·需复核"
		}
		values := []string{fmt.Sprintf("#%03d", i+1), quality,
			strconv.Itoa(len(c.ElementIDs)), strconv.Itoa(c.BodyID),
			fmt.Sprintf("%.2f", c.RadiusMM), fmt.Sprintf("%.2f", c.SizeMM),
			strconv.Itoa(c.Proxy), state}
		first := lvItem{Mask: LVIF_TEXT, Item: int32(i), SubItem: 0, Text: ptr(values[0])}
		send(h, LVM_INSERTITEMW, 0, uintptr(unsafe.Pointer(&first)))
		for column := 1; column < len(values); column++ {
			item := lvItem{Item: int32(i), SubItem: int32(column), Text: ptr(values[column])}
			send(h, LVM_SETITEMTEXTW, uintptr(i), uintptr(unsafe.Pointer(&item)))
		}
	}
	a.reviewSelectionSummary()
}

func (a *state) reviewChecked(i int) bool {
	if a.ctl("reviewlist") == 0 {
		return false
	}
	state := send(a.ctl("reviewlist"), LVM_GETITEMSTATE, uintptr(i), LVIS_STATEIMAGEMASK)
	return (state>>12)&0xF == 2
}
func (a *state) reviewUncheck(i int) {
	item := lvItem{State: 1 << 12, StateMask: LVIS_STATEIMAGEMASK}
	send(a.ctl("reviewlist"), LVM_SETITEMSTATE, uintptr(i), uintptr(unsafe.Pointer(&item)))
}
func (a *state) selectedCandidates() ([]string, int) {
	var ids []string
	budget := a.reviewExistingBudget
	for i, c := range a.reviewPlan.Clusters {
		if !a.reviewChecked(i) {
			continue
		}
		if a.reviewPublished[c.ID] {
			a.reviewUncheck(i)
			continue
		}
		ids = append(ids, c.ID)
		budget += c.Proxy
	}
	return ids, budget
}
func (a *state) existingSphereRelation(candidate WarningHotspot) (nearDuplicate, overlaps bool) {
	for _, old := range a.reviewExistingControls {
		if old.BodyID != candidate.BodyID {
			continue
		}
		other := WarningHotspot{BodyID: old.BodyID, CentreMM: old.CentreMM, RadiusMM: old.RadiusMM}
		distance := candidateCentersDistance(candidate, other)
		if distance < 0.25*math.Min(candidate.RadiusMM, old.RadiusMM) {
			nearDuplicate = true
		}
		if distance < candidate.RadiusMM+old.RadiusMM {
			overlaps = true
		}
	}
	return nearDuplicate, overlaps
}

func (a *state) selectedExistingSphereOverlap(ids []string) (overlapCount int, nearDuplicate bool) {
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	for _, candidate := range a.reviewPlan.Clusters {
		if !selected[candidate.ID] {
			continue
		}
		near, overlap := a.existingSphereRelation(candidate)
		if near {
			nearDuplicate = true
		}
		if overlap {
			overlapCount++
		}
	}
	return overlapCount, nearDuplicate
}

func (a *state) reviewSelectionSummary() {
	if a.page != 5 || a.reviewPopulating {
		return
	}
	ids, budget := a.selectedCandidates()
	// Report overlaps conservatively; ordinary intersections need manual
	// confirmation, near-duplicates are rejected before ACT mutation.
	overlapCount, overlapErr := reviewedSphereOverlap(a.reviewPlan, ids, a.reviewPublished)
	existingOverlapCount, existingNearDuplicate := a.selectedExistingSphereOverlap(ids)
	a.reviewOverlapCount = overlapCount
	a.reviewExistingOverlapCount = existingOverlapCount
	a.reviewExistingNearDuplicate = existingNearDuplicate
	a.reviewOverlapError = overlapErr != nil || existingNearDuplicate
	signature := strings.Join(ids, "|")
	if signature != a.reviewSelectedSignature {
		send(a.ctl("reviewrisk"), BM_SETCHECK, 0, 0)
		a.reviewSelectedSignature = signature
	}
	a.reviewSelectedCount = len(ids)
	a.reviewSelectedBudget = budget
	enabled(a.ctl("reviewconfirm"), !a.busy && a.reviewPublishBlockedReason == "" && len(ids) > 0 && budget <= 120000 && !a.reviewOverlapError && a.reviewDetailsOpened && send(a.ctl("reviewrisk"), BM_GETCHECK, 0, 0) == 1)
	refresh()
}
func (a *state) openCandidateReview() {
	if a.busy {
		alert("当前 ACT 任务仍在运行，不能重新选择发布项目")
		return
	}
	if a.snapshot == nil {
		alert("请先只读扫描当前 Meshing，再生成候选方案")
		return
	}
	plan, err := a.ws.ReadReviewedWarningPlan(a.snapshot)
	if err != nil {
		alert("当前没有可审核的局部方案：" + err.Error())
		return
	}
	for _, c := range plan.Clusters {
		if len(c.ID) != 20 {
			alert("当前预览由旧版算法生成，没有稳定候选 ID。请重新点击「生成局部方案」，再打开人工审核列表。")
			return
		}
	}
	if a.ctl("reviewlist") == 0 {
		alert("原生候选列表控件初始化失败，禁止在没有勾选界面的情况下发布 Sizing。")
		return
	}
	published, budget, ledgerErr := a.ws.warningOwnedCandidates(a.snapshot)
	existingControls, controlsKnown, controlsErr := snapshotWarningControls(a.snapshot)
	publishBlockedReason := ""
	if controlsErr != nil {
		publishBlockedReason = controlsErr.Error()
	} else if !controlsKnown {
		publishBlockedReason = "当前只读扫描未提供完整的现有 Sizing 清单，无法核验历史资源预算。请重新只读扫描。"
	}
	if ledgerErr != nil {
		// A stale or unreadable ledger must not hide a valid read-only proposal.
		// Keep the candidate list available, but fail closed for all publication.
		if publishBlockedReason == "" {
			publishBlockedReason = ledgerErr.Error()
		}
		published = map[string]bool{}
		budget = 0
	} else if publishBlockedReason == "" {
		if err := a.ws.reconcileReviewLegacyPublished(a.snapshot, plan, published); err != nil {
			publishBlockedReason = err.Error()
		}
	}
	a.reviewPlan = plan
	a.reviewExistingControls = existingControls
	a.reviewExistingControlCount = len(existingControls)
	a.reviewPublishBlockedReason = publishBlockedReason
	a.reviewDetailsOpened = false
	a.reviewSelectedSignature = ""
	a.reviewExistingOverlapCount = 0
	a.reviewExistingNearDuplicate = false
	send(a.ctl("reviewrisk"), BM_SETCHECK, 0, 0)
	a.reviewPublished = published
	a.reviewExistingBudget = budget
	a.switchPage(5)
	a.populateCandidateList()
	a.reviewSelectionSummary()
	if a.reviewPublishBlockedReason != "" {
		alert("当前局部方案已生成，可以继续查看全部影响域和候选数据。\n\n但旧版 Sizing 发布记录无法与当前工程安全对应，因此本次进入只读审核模式，发布按钮已锁定；不会删除、覆盖或停用任何现有 Sizing。\n\n原因：" + a.reviewPublishBlockedReason)
	}
}
func (a *state) confirmCandidateReview() {
	if a.reviewPublishBlockedReason != "" {
		alert("当前仅允许查看局部方案，不能发布 Sizing。\n\n" + a.reviewPublishBlockedReason)
		return
	}
	if a.busy {
		alert("任务执行中，不能再次发布")
		return
	}
	ids, budget := a.selectedCandidates()
	if !a.reviewDetailsOpened || send(a.ctl("reviewrisk"), BM_GETCHECK, 0, 0) != 1 {
		alert("请先打开并核查具体影响范围，再勾选人工风险确认。")
		return
	}
	if len(ids) == 0 {
		alert("请勾选至少一个未发布的候选区域")
		return
	}
	if a.reviewExistingNearDuplicate {
		alert("所选候选与当前已有 Sizing 的球形影响域近乎重合，已禁止重复发布。请取消这些候选；其他不重合候选仍可单独发布。")
		return
	}
	if _, err := reviewedSphereOverlap(a.reviewPlan, ids, a.reviewPublished); err != nil {
		alert("候选影响域冲突：" + err.Error())
		return
	}
	if budget > 120000 {
		alert(fmt.Sprintf("累计预算代理约 %d，已超过 120000；减少勾选后再试。", budget))
		return
	}
	if _, err := a.ws.saveWarningSelection(a.snapshot, a.reviewPlan, ids); err != nil {
		alert("未通过提交前安全核对：" + err.Error())
		return
	}
	a.message(fmt.Sprintf("已审核选择 %d 个区域，预算代理约 %d；正在请求最终发布确认", len(ids), budget))
	// a.run performs the final existing Workbench/ACT safety confirmation.
	a.run("warning_refine_publish")
}
func (a *state) openCandidateReviewDetails() {
	path, err := a.ws.SaveWarningPlanHTML(a.snapshot, a.reviewPlan.PlanID)
	if err != nil {
		alert("无法验证影响域详情：" + err.Error())
		return
	}
	if e := exec.Command("cmd", "/c", "start", "", path).Start(); e != nil {
		alert("无法打开候选影响域：" + e.Error())
		return
	}
	a.reviewDetailsOpened = true
	a.reviewSelectionSummary()
}

func (a *state) drawCandidateReview(dc uintptr, x, cw int) {
	textAt(dc, "局部优化候选 · 勾选审核", x+16, 84, cw-32, 43, 25, 700, navy)
	verified := 0
	for _, c := range a.reviewPlan.Clusters {
		if c.SkewVerified && c.SkewMax != nil {
			verified++
		}
	}
	qualityNote := fmt.Sprintf("已核验实测最大偏度：%d/%d；可读取的候选按最大偏度降序。", verified, len(a.reviewPlan.Clusters))
	if verified == 0 {
		qualityNote = "ACT 未提供逐单元偏度数值：仅按缺陷数量辅助排序，不冒充实测偏度。"
	}
	textAt(dc, qualityNote, x+16, 134, cw-32, 27, 13, 450, slate)
	solid(dc, x, 184, cw, 516, white, 18)
	existing := a.reviewExistingControlCount
	historyLabel := fmt.Sprintf("当前模型已有历史 Sizing %d 处（已读取）", existing)
	if a.reviewPublishBlockedReason != "" {
		historyLabel = "历史发布状态：待人工核对"
	}
	textAt(dc, fmt.Sprintf("候选 %d 处  ·  %s  ·  本次勾选 %d 处",
		len(a.reviewPlan.Clusters), historyLabel, a.reviewSelectedCount), x+30, 193, cw-60, 29, 17, 700, navy)
	if a.reviewPublishBlockedReason != "" {
		textAt(dc, "资源预算：旧控件清单未核验，不计算累计值；发布已锁定。", x+30, 226, cw-60, 24, 12, 600, color(180, 75, 45))
		textAt(dc, "只读模式：历史 Sizing 清单未能安全匹配当前模型；可查看候选，不可发布。", x+30, 250, cw-60, 18, 11, 650, color(180, 75, 45))
	} else {
		textAt(dc, fmt.Sprintf("资源代理：本次加已有约 %d / 120000（不是实际新增单元数）", a.reviewSelectedBudget), x+30, 226, cw-60, 24, 12, 450, slate)
	}
	overlapNote := "逐项核查片间净空、几何作用域；默认不勾选任何候选。历史 Sizing 保留并计入累计预算。"
	if a.reviewExistingNearDuplicate {
		overlapNote = "所选候选与历史 Sizing 近乎重合，禁止重复发布；请取消对应候选。"
	} else if a.reviewExistingOverlapCount > 0 {
		overlapNote = fmt.Sprintf("所选候选与已有 Sizing 影响域相交 %d 处；请查看详情并人工核查。", a.reviewExistingOverlapCount)
	} else if a.reviewOverlapError {
		overlapNote = "所选候选之间存在几乎重合的影响球，已禁止发布。"
	} else if a.reviewOverlapCount > 0 {
		overlapNote = fmt.Sprintf("所选影响球存在 %d 对空间相交；请在详情中人工核查。", a.reviewOverlapCount)
	}
	textAt(dc, overlapNote, x+30, 606, cw-60, 21, 12, 500, slate)
	textAt(dc, "仅创建/复用 Sizing，不清网格、不执行 GenerateMesh；未勾选候选会保留。", x+30, 679, cw-60, 20, 11, 450, slate)
}
