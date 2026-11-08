package main

// All control rectangles and critical label rectangles share a single layout map.
// Minimum supported logical client size is 1100x690. Vertical layout fits 700 px.
const (
	contentX        = 220
	mainTop         = 80
	homeSummaryY    = 244
	homeLowerY      = 439
	homeButtonsY    = 586
	connectCardY    = 179
	connectButtonsY = 307
	reportButtonsY  = 564
	settingsExportY = 313
)

// Keep every column and its labels on the same coordinate grid. In previous
// versions the listbox width grew with the window while the right-hand labels
// remained at x+585, causing them to render behind the list.
func fineColumns(width int) (listW, rightX, rightW int) {
	cw := uiWidth(width)
	listW = cw * 48 / 100
	if listW < 390 {
		listW = 390
	}
	if listW > 570 {
		listW = 570
	}
	rightX = contentX + 30 + listW + 28
	rightW = contentX + cw - 30 - rightX
	return
}

type uiBox struct{ X, Y, W, H int }

func (a uiBox) intersects(b uiBox) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}
func uiWidth(width int) int {
	if width-240 < 860 {
		return 860
	}
	return width - 240
}
func plannedControls(width int) map[string]uiBox {
	cw := uiWidth(width)
	x := contentX
	gap := 12
	col := (cw - 2*gap) / 3
	x3 := x + 2*(col+gap)
	left := cw * 63 / 100
	listW, rightX, rightW := fineColumns(width)
	return map[string]uiBox{
		"navhome": {16, 104, 168, 43}, "navconn": {16, 151, 168, 43}, "navfine": {16, 198, 168, 43}, "navreport": {16, 245, 168, 43}, "navsettings": {16, 292, 168, 43},
		"auto": {x + 28, 181, 307, 43},
		// New direct-to-existing-NS workflow: visible, scrollable named list.
		"finegap": {x + 30, 237, 224, 28}, "finegapmm": {x + 276, 237, 123, 30},
		"finehole": {x + 30, 322, 224, 28}, "fineholemm": {x + 276, 322, 123, 30},
		"fineport": {x + 30, 407, 224, 28}, "fineportmm": {x + 276, 407, 123, 30},
		"finecurve": {x + 30, 492, 224, 28}, "finecurvemm": {x + 276, 492, 123, 30},
		"finegroups":      {x + 30, 307, listW, 238},
		"finefilter":      {x + 30, 270, 120, 29},
		"finerefresh":     {x + 160, 270, 105, 29},
		"finechinese":     {x + 275, 270, 130, 29},
		"finegroupactive": {rightX, 387, rightW, 28},
		"finegroupmm":     {rightX, 461, 126, 30},
		"finegroupsave":   {rightX, 505, 160, 32},
		"finesuggest":     {x + 30, 550, 177, 30},
		"finebulkmm":      {rightX, 559, 95, 30},
		"finebulkapply":   {rightX + 107, 559, 116, 30},
		// Audit global setup BEFORE publishing or regenerating a mesh.
		"fineglobalreview": {x + 30, 649, 172, 32},
		"finenames":        {x + 214, 649, 170, 32},
		"fineprepare":      {x + 396, 649, 205, 32},
		"finegenerate":     {x + 613, 649, 217, 32},
		"size":             {x3 + 20, 326, col - 40, 34}, "global": {x3 + 20, 365, col - 40, 28},
		"scan": {x + 22, homeButtonsY, 132, 36}, "report": {x + 166, homeButtonsY, 132, 36}, "open": {x + 310, homeButtonsY, 165, 36},
		"link": {x + 30, 307, 230, 38}, "port": {x + 284, 307, 122, 38},
		"initretry": {x + 423, 307, 185, 38}, "taskstatusconn": {x + 620, 307, 195, 38},
		"systems": {x + 30, 443, 420, 190}, "ack": {x + 30, 501, 540, 32},
		// Report page follows the real sequence: inspect -> mark interval -> preview -> view -> publish.
		"qualitycheck": {x + 30, 483, 170, 34}, "skewlow": {x + 296, 485, 70, 30}, "skewhigh": {x + 395, 485, 70, 30},
		"markhighskew": {x + 486, 483, 188, 34}, "markwarning": {x + 686, 483, 144, 34},
		"warningplan": {x + 30, 568, 205, 34}, "warningview": {x + 245, 568, 160, 34},
		"warningpublish": {x + 415, 568, 190, 34}, "taskstatus": {x + 615, 568, 198, 34},
		"htmlreport": {x + 30, 633, 175, 34}, "diagnose": {x + 215, 633, 165, 34}, "mark": {x + 390, 633, 150, 34},
		"reviewlist":      {x + 30, 268, cw - 60, 325},
		"reviewback":      {x + 30, 651, 170, 34},
		"reviewclear":     {x + 212, 651, 140, 34},
		"reviewdetails":   {x + 364, 651, 180, 34},
		"reviewconfirm":   {x + cw - 258, 651, 228, 34},
		"export":          {x + 30, settingsExportY, cw - 60, 38},
		"exportbutton":    {x + 30, 369, 282, 36},
		"clearlease":      {x + 30, 596, 265, 35},
		"tasklog":         {x + 310, 596, 190, 35},
		"storagechange":   {x + 30, 525, 230, 35},
		"_home_left_card": {x, homeLowerY, left, 192}, "_home_right_card": {x + left + gap, homeLowerY, cw - left - gap, 192},
	}
}

type pageLabel struct {
	Name string
	Page int
	Area uiBox
}

func criticalLabels(width int) []pageLabel {
	cw := uiWidth(width)
	x := contentX
	gap := 12
	col := (cw - 2*gap) / 3
	x3 := x + 2*(col+gap)
	left := cw * 63 / 100
	listW, rightX, rightW := fineColumns(width)
	return []pageLabel{
		{"主页说明", 0, uiBox{x + 360, 178, cw - 385, 37}},
		{"尺寸标题", 0, uiBox{x3 + 20, 299, col - 40, 24}},
		{"尺寸后说明", 0, uiBox{x3 + 20, 398, col - 40, 22}},
		{"运行状态", 0, uiBox{x + 20, homeLowerY + 54, left - 45, 27}},
		{"运行详情", 0, uiBox{x + 20, homeLowerY + 86, left - 45, 24}},
		{"局部页面标题", 2, uiBox{x + 16, 83, cw - 32, 43}},
		{"命名列表标题", 2, uiBox{x + 30, 202, listW, 27}},
		{"命名列表说明", 2, uiBox{x + 30, 231, listW, 23}},
		{"右侧选中名称", 2, uiBox{rightX, 249, rightW, 43}},
		{"右侧面数量", 2, uiBox{rightX, 299, rightW, 24}},
		{"右侧启用提示", 2, uiBox{rightX, 355, rightW, 24}},
		{"右侧尺寸提示", 2, uiBox{rightX, 434, rightW, 24}},
		{"右侧保存提示", 2, uiBox{rightX, 537, rightW, 18}},
		{"已配置说明", 2, uiBox{x + 215, 555, listW - 215, 25}},
		{"命名底部说明", 2, uiBox{x + 30, 591, cw - 60, 16}},
		{"网格预算提示", 2, uiBox{x + 30, 609, cw - 60, 16}},
		{"网格预算说明", 2, uiBox{x + 30, 629, cw - 60, 16}},
		{"连接端口标题", 1, uiBox{x + 284, 273, 245, 26}},
		{"网格标题", 1, uiBox{x + 30, 376, cw - 60, 30}},
		{"网格名称标签", 1, uiBox{x + 30, 414, cw - 60, 22}},
		{"编辑器提示", 1, uiBox{x + 30, 544, cw - 60, 26}},
		{"连接状态", 1, uiBox{x + 30, 596, cw - 60, 27}},
		{"质量筛选步骤标题", 3, uiBox{x + 30, 449, cw - 60, 24}},
		{"偏度范围标题", 3, uiBox{x + 211, 485, 82, 30}},
		{"偏度区间连接", 3, uiBox{x + 371, 485, 20, 30}},
		{"方案步骤标题", 3, uiBox{x + 30, 535, cw - 60, 25}},
		{"辅助工具步骤标题", 3, uiBox{x + 30, 608, cw - 60, 22}},
		{"导出填写提示", 4, uiBox{x + 30, 257, cw - 60, 27}},
		{"导出后说明", 4, uiBox{x + 350, 369, cw - 380, 36}},
	}
}

// Scope-independent UI preference is safe to retain only when the underlying
// project/geometry identity is unchanged AND the name still exists.
// Actual current scoping is always re-verified by Meshing before publishing.
func preservedNamedSizes(previous map[string]float64, previousIdentity, currentIdentity string, names []Named) map[string]float64 {
	current := make(map[string]float64)
	if previousIdentity == "" || previousIdentity != currentIdentity {
		return current
	}
	for _, n := range names {
		if len(n.IDs) == 0 {
			continue
		}
		if value, exists := previous[n.Name]; exists {
			current[n.Name] = value
		}
	}
	return current
}
