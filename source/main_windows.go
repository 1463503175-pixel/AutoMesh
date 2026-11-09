//go:build windows

// AutoMesh R6 Integrated - existing-session Workbench scripting bridge.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	WS_VISIBLE                   = 0x10000000
	WS_CHILD                     = 0x40000000
	WS_BORDER                    = 0x00800000
	WS_TABSTOP                   = 0x00010000
	WS_OVERLAPPEDWINDOW          = 0x00CF0000
	WS_VSCROLL                   = 0x00200000
	WS_HSCROLL                   = 0x00100000
	ES_AUTOHSCROLL               = 0x0080
	ES_MULTILINE                 = 0x0004
	ES_AUTOVSCROLL               = 0x0040
	ES_READONLY                  = 0x0800
	BS_AUTOCHECKBOX              = 0x0003
	BS_OWNERDRAW                 = 0x000B
	CBS_DROPDOWNLIST             = 0x0003
	WM_DESTROY                   = 0x0002
	WM_GETMINMAXINFO             = 0x0024
	WM_SIZE                      = 0x0005
	WM_MOUSEWHEEL                = 0x020A
	WM_VSCROLL                   = 0x0115
	WM_KEYDOWN                   = 0x0100
	VK_TAB                       = 0x0009
	EN_CHANGE                    = 0x0300
	WM_HSCROLL                   = 0x0114
	SB_HORZ                      = 0
	SB_VERT                      = 1
	SB_LINEUP                    = 0
	SB_LINEDOWN                  = 1
	SB_PAGEUP                    = 2
	SB_PAGEDOWN                  = 3
	SB_THUMBPOSITION             = 4
	SB_THUMBTRACK                = 5
	SB_TOP                       = 6
	SB_BOTTOM                    = 7
	SIF_ALL                      = 0x17
	WM_ENTERSIZEMOVE             = 0x0231
	WM_EXITSIZEMOVE              = 0x0232
	WM_PAINT                     = 0x000F
	WM_ERASEBKGND                = 0x0014
	WM_COMMAND                   = 0x0111
	WM_DRAWITEM                  = 0x002B
	WM_CTLCOLORSTATIC            = 0x0138
	WM_CTLCOLOREDIT              = 0x0133
	WM_CTLCOLORBTN               = 0x0135
	WM_SETFONT                   = 0x0030
	WM_APPSTATUS                 = 0x8001
	WM_APPDONE                   = 0x8002
	WM_APPINIT                   = 0x8003
	WM_APPSTORAGE                = 0x8004
	BM_GETCHECK                  = 0x00F0
	BM_SETCHECK                  = 0x00F1
	CB_ADDSTRING                 = 0x0143
	CB_SETCURSEL                 = 0x014E
	CB_GETCURSEL                 = 0x0147
	CB_RESETCONTENT              = 0x014B
	LB_ADDSTRING                 = 0x0180
	LB_RESETCONTENT              = 0x0184
	LB_GETCURSEL                 = 0x0188
	LB_SETCURSEL                 = 0x0186
	LBS_NOTIFY                   = 0x0001
	LBS_NOINTEGRALHEIGHT         = 0x0100
	MB_YESNO                     = 4
	MB_ICONWARNING               = 0x30
	MB_ICONERROR                 = 0x10
	MB_ICONINFORMATION           = 0x40
	ODS_SELECTED                 = 0x0001
	BN_CLICKED                   = 0x0000
	CBN_SELCHANGE                = 0x0001
	ODS_DISABLED                 = 0x0004
	ODS_FOCUS                    = 0x0010
	DT_CENTER                    = 0x1
	DT_VCENTER                   = 0x4
	DT_SINGLELINE                = 0x20
	DT_END_ELLIPSIS              = 0x8000
	ID_NAV_HOME                  = 100
	ID_NAV_CONNECT               = 101
	ID_NAV_FINE                  = 102
	ID_NAV_REPORT                = 103
	ID_NAV_ANALYSIS              = 104
	ID_NAV_SETTINGS              = 105
	ID_AUTO                      = 110
	ID_SCAN                      = 111
	ID_CONNECT                   = 112
	ID_REPORT                    = 113
	ID_OPEN                      = 114
	ID_COPY                      = 115
	ID_HTML_REPORT               = 116
	ID_DIAGNOSE                  = 117
	ID_MARK                      = 118
	ID_FINE_GENERATE             = 119
	ID_RECOVER                   = 120
	ID_CLEAR_LEASE               = 121
	ID_FINE_PREPARE              = 122
	ID_EXPORT_MSH                = 123
	ID_FINE_GROUP_SELECT         = 124
	ID_FINE_GROUP_SAVE           = 125
	ID_FINE_NAMES                = 126
	ID_FINE_FILTER               = 127
	ID_FINE_REFRESH              = 128
	ID_QUALITY_CHECK             = 129
	ID_CHINESE_NAMES             = 130
	ID_FINE_AUTO_RECOMMEND       = 131
	ID_FINE_BULK_SET             = 132
	ID_MARK_WARNING              = 133
	ID_STORAGE_CHANGE            = 134
	ID_WARNING_PLAN              = 135
	ID_WARNING_PUBLISH           = 136
	ID_WARNING_VIEW              = 137
	ID_FINE_GLOBAL_REVIEW        = 138
	ID_TASK_STATUS               = 139
	ID_INIT_RETRY                = 140
	ID_TASK_STOP                 = 141
	ID_MARK_HIGH_SKEW            = 142
	ID_SYNC_STATUS               = 147
	ID_TASK_CENTER               = 148
	ID_ANALYSIS_REFRESH          = 149
	ID_ANALYSIS_EXPORT           = 150
	ID_ANALYSIS_TAB_OVERVIEW     = 151
	ID_ANALYSIS_TAB_COMPARE      = 152
	ID_ANALYSIS_TAB_TREND        = 153
	ID_ANALYSIS_TAB_DISTRIBUTION = 154
	ID_ANALYSIS_TAB_REGION       = 155
	ID_TASK_DRAWER_CLOSE         = 156
	ID_MARK_EXECUTE              = 157
	ID_TASK_DRAWER_LOG           = 158
	ID_ANALYSIS_DEEP_QUALITY     = 159
	ID_ANALYSIS_REPORT           = 160
	ID_ACTION_HELP               = 161
	ID_REVIEW_SELECT_ALL         = 162
	ID_REVIEW_SELECT_SEVERE      = 163
	ID_REVIEW_SELECT_WARNING     = 164
	ID_REVIEW_SELECT_OTHER       = 165
	ID_REVIEW_WHY_DISABLED       = 166
	// Reserved IDs for hidden legacy HWNDs. They are deliberately outside the
	// active action-ID range so an accidental visibility regression cannot
	// invoke a second route for a global action.
	ID_LEGACY_QUALITY_CHECK    = 170
	ID_LEGACY_TASK_STATUS_CONN = 171
	ID_LEGACY_TASK_STATUS      = 172
	ID_LEGACY_TASK_LOG         = 173
	ID_LEGACY_FINE_BULK_SET    = 174
)

var (
	usr           = syscall.NewLazyDLL("user32.dll")
	gdi           = syscall.NewLazyDLL("gdi32.dll")
	krn           = syscall.NewLazyDLL("kernel32.dll")
	register      = usr.NewProc("RegisterClassExW")
	create        = usr.NewProc("CreateWindowExW")
	defproc       = usr.NewProc("DefWindowProcW")
	getmsg        = usr.NewProc("GetMessageW")
	isDialogMsg   = usr.NewProc("IsDialogMessageW")
	getFocus      = usr.NewProc("GetFocus")
	translate     = usr.NewProc("TranslateMessage")
	dispatch      = usr.NewProc("DispatchMessageW")
	sendp         = usr.NewProc("SendMessageW")
	post          = usr.NewProc("PostMessageW")
	settext       = usr.NewProc("SetWindowTextW")
	gettext       = usr.NewProc("GetWindowTextW")
	msgbox        = usr.NewProc("MessageBoxW")
	loadcursor    = usr.NewProc("LoadCursorW")
	loadicon      = usr.NewProc("LoadIconW")
	show          = usr.NewProc("ShowWindow")
	enableWindow  = usr.NewProc("EnableWindow")
	update        = usr.NewProc("UpdateWindow")
	invalidate    = usr.NewProc("InvalidateRect")
	quit          = usr.NewProc("PostQuitMessage")
	getrect       = usr.NewProc("GetClientRect")
	setScrollInfo = usr.NewProc("SetScrollInfo")
	getScrollInfo = usr.NewProc("GetScrollInfo")
	setWindowRgn  = usr.NewProc("SetWindowRgn")
	beginpaint    = usr.NewProc("BeginPaint")
	endpaint      = usr.NewProc("EndPaint")
	module        = krn.NewProc("GetModuleHandleW")
	memstatus     = krn.NewProc("GlobalMemoryStatusEx")
	createMutex   = krn.NewProc("CreateMutexW")
	closeHandle   = krn.NewProc("CloseHandle")
	setDpiContext = usr.NewProc("SetProcessDpiAwarenessContext")
	createfont    = gdi.NewProc("CreateFontW")
	brush         = gdi.NewProc("CreateSolidBrush")
	pen           = gdi.NewProc("CreatePen")
	fill          = usr.NewProc("FillRect")
	roundrect     = gdi.NewProc("RoundRect")
	rectangle     = gdi.NewProc("Rectangle")
	moveto        = gdi.NewProc("MoveToEx")
	lineto        = gdi.NewProc("LineTo")
	createRectRgn = gdi.NewProc("CreateRectRgn")
	saveDC        = gdi.NewProc("SaveDC")
	restoreDC     = gdi.NewProc("RestoreDC")
	intersectClip = gdi.NewProc("IntersectClipRect")
	setViewOrigin = gdi.NewProc("SetViewportOrgEx")
	textcolor     = gdi.NewProc("SetTextColor")
	bkmode        = gdi.NewProc("SetBkMode")
	selectobj     = gdi.NewProc("SelectObject")
	deleteobj     = gdi.NewProc("DeleteObject")
	drawtext      = usr.NewProc("DrawTextW")
	newdc         = gdi.NewProc("CreateCompatibleDC")
	newbmp        = gdi.NewProc("CreateCompatibleBitmap")
	blit          = gdi.NewProc("BitBlt")
	deldc         = gdi.NewProc("DeleteDC")
)

type wndClass struct {
	Size, Style                                               uint32
	Proc                                                      uintptr
	ClassExtra, WindowExtra                                   int32
	Instance, Icon, Cursor, Background, Menu, Name, SmallIcon uintptr
}
type winMsg struct {
	Window  uintptr
	Msg     uint32
	W, L    uintptr
	Time    uint32
	X, Y    int32
	Private uint32
}
type rect struct{ L, T, R, B int32 }
type scrollInfo struct {
	Size, Mask    uint32
	Min, Max      int32
	Page          uint32
	Pos, TrackPos int32
}
type paintStruct struct {
	HDC                uintptr
	Erase              int32
	RC                 rect
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type drawItem struct {
	Type, ID, ItemID, Action, State uint32
	HWND, HDC                       uintptr
	RC                              rect
	Data                            uintptr
}

var backDC, backBitmap, backOldBitmap uintptr

// Grow the bitmap in buckets; do not allocate on every intermediate window size.
var backWidth, backHeight int
var backBitmapAllocations int
var layoutMoves int
var uiRepaints int
var paintDurationMax time.Duration
var layoutDurationMax time.Duration
var navigationDurationMax time.Duration
var startedAt = time.Now()
var firstPaintMs float64 = -1
var resizeActive bool

func backBuffer(dc uintptr, w, h int) uintptr {
	if w <= 0 || h <= 0 {
		return 0
	}
	if backDC != 0 && w <= backWidth && h <= backHeight {
		return backDC
	}
	if backDC != 0 {
		call(selectobj, backDC, backOldBitmap)
		call(deleteobj, backBitmap)
		call(deldc, backDC)
	}
	// Capacity rounds up independently, avoiding allocate/free storms during drag.
	allocW := ((w + 255) / 256) * 256
	allocH := ((h + 127) / 128) * 128
	backDC = call(newdc, dc)
	backBitmap = call(newbmp, dc, uintptr(allocW), uintptr(allocH))
	if backDC == 0 || backBitmap == 0 {
		return 0
	}
	backOldBitmap = call(selectobj, backDC, backBitmap)
	backWidth, backHeight = allocW, allocH
	backBitmapAllocations++
	return backDC
}
func disposeBuffer() {
	if backDC != 0 {
		call(selectobj, backDC, backOldBitmap)
		call(deleteobj, backBitmap)
		call(deldc, backDC)
	}
	backDC = 0
	backBitmap = 0
	backWidth, backHeight = 0, 0
}

type memoryStatus struct {
	Length, Load                                                             uint32
	Total, Avail, TotalPage, AvailPage, TotalVirtual, AvailVirtual, Extended uint64
}

func call(p *syscall.LazyProc, args ...uintptr) uintptr { v, _, _ := p.Call(args...); return v }
func wide(s string) *uint16                             { return syscall.StringToUTF16Ptr(s) }
func color(r, g, b int) uintptr                         { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }

var (
	navy         = color(27, 43, 70)
	slate        = color(98, 113, 138)
	cobalt       = color(24, 94, 225)
	bluepale     = color(231, 241, 255)
	white        = color(255, 255, 255)
	panelbg      = color(246, 248, 252)
	lineclr      = color(229, 234, 243)
	green        = color(24, 144, 99)
	greenbg      = color(230, 247, 238)
	darkbg       = color(235, 240, 250)
	warningcolor = color(145, 93, 17)
	focusblue    = color(24, 94, 225)
	stoppale     = color(252, 241, 241)
	stopink      = color(158, 60, 60)
)

// Cached, UI-thread-only GDI objects. Never dispose these inside WM_PAINT.
type fontKey struct{ size, weight int }

var gdiFonts = map[fontKey]uintptr{}
var gdiBrushes = map[uintptr]uintptr{}
var gdiPens = map[uintptr]uintptr{}

func cachedFont(size, weight int) uintptr {
	key := fontKey{size, weight}
	if f := gdiFonts[key]; f != 0 {
		return f
	}
	f := call(createfont, uintptr(int32(-size)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, ptr("Microsoft YaHei UI"))
	gdiFonts[key] = f
	return f
}
func cachedBrush(c uintptr) uintptr {
	if h := gdiBrushes[c]; h != 0 {
		return h
	}
	h := call(brush, c)
	gdiBrushes[c] = h
	return h
}
func cachedPen(c uintptr) uintptr {
	if h := gdiPens[c]; h != 0 {
		return h
	}
	h := call(pen, 0, 1, c)
	gdiPens[c] = h
	return h
}
func releaseGDI() {
	for _, p := range gdiFonts {
		call(deleteobj, p)
	}
	for _, p := range gdiBrushes {
		call(deleteobj, p)
	}
	for _, p := range gdiPens {
		call(deleteobj, p)
	}
}
func ptr(s string) uintptr { return uintptr(unsafe.Pointer(wide(s))) }
func txt(hdc uintptr, s string, box rect, size, weight int, fg uintptr, flags uint32) {
	f := cachedFont(size, weight)
	prev := call(selectobj, hdc, f)
	call(textcolor, hdc, fg)
	call(bkmode, hdc, 1)
	call(drawtext, hdc, ptr(s), ^uintptr(0), uintptr(unsafe.Pointer(&box)), uintptr(flags))
	call(selectobj, hdc, prev)
}
func textAt(dc uintptr, s string, x, y, w, h, size, weight int, fg uintptr) {
	txt(dc, s, rect{int32(x), int32(y), int32(x + w), int32(y + h)}, size, weight, fg, DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
}
func solid(dc uintptr, x, y, w, h int, c uintptr, radius int) {
	if w <= 0 || h <= 0 {
		return
	}
	oldbr := call(selectobj, dc, cachedBrush(c))
	oldpn := call(selectobj, dc, cachedPen(c))
	if radius == 0 {
		call(rectangle, dc, uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h))
	} else {
		call(roundrect, dc, uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h), uintptr(radius), uintptr(radius))
	}
	call(selectobj, dc, oldpn)
	call(selectobj, dc, oldbr)
}
func rule(dc uintptr, x, y, x2, y2 int, c uintptr) {
	old := call(selectobj, dc, cachedPen(c))
	call(moveto, dc, uintptr(x), uintptr(y), 0)
	call(lineto, dc, uintptr(x2), uintptr(y2))
	call(selectobj, dc, old)
}
func send(h uintptr, msg uint32, w, l uintptr) uintptr { return call(sendp, h, uintptr(msg), w, l) }
func set(h uintptr, s string) {
	if h != 0 {
		call(settext, h, ptr(s))
	}
}
func read(h uintptr) string {
	b := make([]uint16, 4096)
	n := call(gettext, h, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b[:n])
}
func control(class, label string, style uint32, x, y, w, height, id int) uintptr {
	hwnd := call(create, 0, ptr(class), ptr(label), uintptr(style|WS_VISIBLE|WS_CHILD), uintptr(x), uintptr(y), uintptr(w), uintptr(height), ui.window, uintptr(id), ui.instance, 0)
	if hwnd != 0 {
		visibility[hwnd] = true
		placements[hwnd] = [4]int{x, y, w, height}
	}
	return hwnd
}
func button(label string, x, y, w, h, id int) uintptr {
	return control("BUTTON", label, BS_OWNERDRAW|WS_TABSTOP, x, y, w, h, id)
}
func edit(label string, x, y, w, h int) uintptr {
	return control("EDIT", label, WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, x, y, w, h, 0)
}
func checkbox(label string, x, y, w, h int) uintptr {
	return control("BUTTON", label, WS_TABSTOP|BS_AUTOCHECKBOX, x, y, w, h, 0)
}
func combo(x, y, w, h int) uintptr {
	// Win32 COMBOBOX height includes the dropdown list, not just the closed field.
	return control("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, x, y, w, maxInt(h, 190), 0)
}
func font(h uintptr, sz, weight int) {
	send(h, WM_SETFONT, cachedFont(sz, weight), 1)
}
func add(h uintptr, s string) { send(h, CB_ADDSTRING, 0, ptr(s)) }

// Native MoveWindow is costly during live resize. Skip identical geometry.
var placements = map[uintptr][4]int{}
var visibility = map[uintptr]bool{}
var moveWindow = usr.NewProc("MoveWindow")

func repoint(h uintptr, x, y, w, hh int) {
	if h == 0 {
		return
	}
	if w < 8 {
		w = 8
	}
	if hh < 8 {
		hh = 8
	}
	newbox := [4]int{x, y, w, hh}
	if old, ok := placements[h]; ok && old == newbox {
		return
	}
	placements[h] = newbox
	layoutMoves++
	call(moveWindow, h, uintptr(x), uintptr(y), uintptr(w), uintptr(hh), 0)
}
func visible(h uintptr, yes bool) {
	if h == 0 {
		return
	}
	if previous, ok := visibility[h]; ok && previous == yes {
		return
	}
	visibility[h] = yes
	mode := uintptr(0)
	if yes {
		mode = 5
	}
	call(show, h, mode)
}
func refresh()        { call(invalidate, ui.window, 0, 0) }
func alert(s string)  { call(msgbox, ui.window, ptr(s), ptr("AutoMesh R8.1"), MB_ICONERROR) }
func notice(s string) { call(msgbox, ui.window, ptr(s), ptr("AutoMesh R8.1"), MB_ICONINFORMATION) }
func enabled(h uintptr, yes bool) {
	if h == 0 {
		return
	}
	value := uintptr(0)
	if yes {
		value = 1
	}
	call(enableWindow, h, value)
}
func confirm(s string) bool {
	return call(msgbox, ui.window, ptr(s), ptr("AutoMesh R8.1"), MB_YESNO|MB_ICONWARNING) == 6
}
func freeGiB() float64 {
	m := memoryStatus{}
	m.Length = uint32(unsafe.Sizeof(m))
	if call(memstatus, uintptr(unsafe.Pointer(&m))) == 0 {
		return -1
	}
	return float64(m.Avail) / (1 << 30)
}

type state struct {
	instance, window              uintptr
	ws                            Workspace
	page                          int
	scrollY                       int
	scrollX                       int
	layoutInProgress              bool
	controls                      map[string]uintptr
	mu                            sync.Mutex
	status, progress, details     string
	progressPercent               int
	errorText                     string
	busy, connected, ready        bool
	stopSignal                    chan struct{}
	stopRequested                 bool
	activePhase                   string
	initRunning                   bool
	autoConnecting                bool
	initError                     string
	initStarted                   time.Time
	initFinished                  time.Time
	systems                       []string
	port                          int
	security                      string
	snapshot                      *Snapshot
	last                          *Result
	lastSummary                   map[string]any
	qualityResult                 *Result
	qualitySummary                map[string]any
	qualityCheckedAt              time.Time
	qualityStale                  bool
	qualityFingerprint            string
	qualityMeshSignature          string
	qualityProject                string
	qualityQuickGeometrySignature string
	shared                        sharedDataRuntime
	engineeringSummary            map[string]any
	engineeringSyncedAt           time.Time
	qualityHistory                []QualitySnapshot
	analysisBaseIndex             int
	analysisTargetIndex           int
	analysisTab                   int
	analysisError                 string
	taskCenterOpen                bool
	taskCenterStatus              string
	taskCenterText                string
	taskCenterStatusUpdatedAt     time.Time
	activeExport                  string
	activeApplyGlobal             bool
	scannedSystem                 string
	scannedBaseMM                 float64
	preparedSystem                string
	proposals                     []map[string]any
	groupKeys                     []string
	groupOverrides                map[string]float64
	groupDisabled                 map[string]bool
	groupEnabled                  map[string]bool
	namedRows                     []Named
	namedVisible                  []Named
	advice                        []NamedAdvice
	sizeBuckets                   []SizeBucket
	bucketCollapsed               map[int]bool
	planCustomized                bool
	showAllNamed                  bool
	namedGeometry                 string
	selectedNamed                 map[string]float64
	pausedNamedSizes              map[string]float64
	history                       []string
	storageDoneDir                string
	storageDoneFiles              int
	storageError                  error
	// R8.0 explicit, reviewed candidate selection: never implicit/all-selected.
	reviewPlan                  WarningPlan
	reviewPublished             map[string]bool
	reviewExistingBudget        int
	reviewExistingControls      []warningControlSnapshot
	reviewExistingControlCount  int
	reviewExistingOverlapCount  int
	reviewExistingNearDuplicate bool
	reviewSelectedBudget        int
	reviewOverlapCount          int
	reviewOverlapError          bool
	reviewSelectedCount         int
	reviewPopulating            bool
	reviewDetailsOpened         bool
	reviewPublishBlockedReason  string
	reviewSelectedSignature     string
	reviewConfirmDisabledReason string
	// Local-only drafts for two opposite-quality interval conventions.
	// Skewness: high is poor; Element Quality: low is poor.
	lastMarkRule   int
	skewRangeDraft [2]string
	eqRangeDraft   [2]string
}

var ui = &state{controls: map[string]uintptr{}, showAllNamed: true, security: "auto", status: "未连接 Workbench", progress: "准备就绪", details: "首先连接已打开的 ANSYS Workbench 2024 R2。"}

func (a *state) ctl(k string) uintptr    { return a.controls[k] }
func (a *state) put(k string, h uintptr) { a.controls[k] = h; font(h, 15, 400) }
func (a *state) message(s string) {
	a.status = s
	a.history = append(a.history, time.Now().Format("15:04:05")+"  "+s)
	if len(a.history) > 80 {
		a.history = a.history[len(a.history)-80:]
	}
	refresh()
}
func (a *state) postProgress(s string) {
	// Internal progress protocol: P<number>|stage|details, or I|stage|details.
	// The percentage denotes completed *measurable stages*, not an ANSYS
	// solver estimate. An indeterminate stage is displayed as "计算中".
	a.mu.Lock()
	parts := strings.SplitN(s, "|", 3)
	if len(parts) == 3 && (strings.HasPrefix(parts[0], "P") || parts[0] == "I") {
		a.progress = parts[1]
		a.details = parts[2]
		a.progressPercent = -1
		if strings.HasPrefix(parts[0], "P") {
			if n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "P")); err == nil && n >= 0 && n <= 100 {
				a.progressPercent = n
			}
		}
	} else {
		a.progress = s
		a.details = s
		a.progressPercent = -1
	}
	a.mu.Unlock()
	call(post, a.window, WM_APPSTATUS, 0, 0)
}
func (a *state) report() map[string]any {
	if a.lastSummary == nil {
		return map[string]any{}
	}
	return a.lastSummary
}
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func printable(v any) string {
	if v == nil {
		return "—"
	}
	switch vv := v.(type) {
	case float64:
		return strconv.FormatFloat(vv, 'f', -1, 64)
	case int:
		return strconv.Itoa(vv)
	case string:
		return vv
	}
	return fmt.Sprint(v)
}
func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (a *state) build() {
	a.put("navhome", button("概览", 16, 104, 168, 43, ID_NAV_HOME))
	a.put("navconn", button("连接工程", 16, 151, 168, 43, ID_NAV_CONNECT))
	a.put("navfine", button("局部细化", 16, 198, 168, 43, ID_NAV_FINE))
	a.put("navreport", button("质量报告", 16, 245, 168, 43, ID_NAV_REPORT))
	a.put("navanalysis", button("质量分析", 16, 292, 168, 43, ID_NAV_ANALYSIS))
	a.put("navsettings", button("设置与维护", 16, 339, 168, 43, ID_NAV_SETTINGS))
	// Global read-only services. Page navigation never starts these tasks.
	a.put("syncstatus", button("同步工程", 520, 14, 84, 34, ID_SYNC_STATUS))
	a.put("qualityupdate", button("更新质量", 612, 14, 92, 34, ID_QUALITY_CHECK))
	a.put("taskcenter", button("任务中心", 712, 14, 92, 34, ID_TASK_CENTER))
	a.put("auto", button("连接 Workbench", 245, 181, 300, 43, ID_AUTO))
	a.put("scan", button("只读扫描", 248, 586, 132, 36, ID_SCAN))
	a.put("report", button("查看报告", 392, 586, 132, 36, ID_REPORT))
	a.put("open", button("打开文件夹", 537, 586, 148, 36, ID_OPEN))
	a.put("port", edit("自动", 250, 306, 180, 38))
	if old, err := readJSON[map[string]any](a.ws.path("last_connection.json")); err == nil {
		if raw, ok := old["port"].(float64); ok && raw >= 1 && raw <= 65535 {
			set(a.ctl("port"), strconv.Itoa(int(raw)))
		}
	}
	font(a.ctl("port"), 16, 450)
	a.put("link", button("自动查找并连接", 447, 306, 220, 38, ID_CONNECT))
	a.put("initretry", button("重新检查初始化", 700, 306, 180, 38, ID_INIT_RETRY))
	a.put("taskstatusconn", button("检查任务状态", 890, 306, 175, 38, ID_LEGACY_TASK_STATUS_CONN))
	a.put("systems", combo(250, 445, 420, 190))
	a.put("ack", checkbox("我已在 Workbench 中打开目标 Meshing 编辑器", 250, 507, 540, 32))
	a.put("size", edit("12", 250, 162, 105, 32))
	a.put("global", checkbox("应用参考尺寸覆盖全局（默认关闭）", 375, 162, 320, 32))
	send(a.ctl("global"), BM_SETCHECK, 0, 0)
	for _, spec := range []struct{ key, label string }{
		{"gap", "细小相似面"}, {"hole", "重复面积面组"}, {"port", "已有人工命名面"}, {"curve", "其他较大面（默认关闭）"},
	} {
		a.put("fine"+spec.key, checkbox(spec.label, 251, 245, 245, 32))
		send(a.ctl("fine"+spec.key), BM_SETCHECK, 1, 0)
		a.put("fine"+spec.key+"mm", edit("", 510, 245, 120, 33))
	}
	a.put("finenames", button("自动命名（可选）", 252, 613, 184, 36, ID_FINE_NAMES))
	a.put("fineprepare", button("发布命名 Sizing", 450, 613, 184, 36, ID_FINE_PREPARE))
	a.put("finegenerate", button("审核后生成网格", 649, 641, 195, 34, ID_FINE_GENERATE))
	a.put("fineglobalreview", button("核对全局参数", 895, 641, 155, 34, ID_FINE_GLOBAL_REVIEW))
	a.put("finegroups", control("LISTBOX", "", WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY|LBS_NOINTEGRALHEIGHT, 250, 307, 510, 232, ID_FINE_GROUP_SELECT))
	a.put("finefilter", button("只看 AutoMesh", 250, 266, 165, 30, ID_FINE_FILTER))
	a.put("finerefresh", button("完整几何扫描", 428, 266, 185, 30, ID_FINE_REFRESH))
	a.put("finechinese", button("旧版辅助命名中文化", 625, 266, 210, 35, ID_CHINESE_NAMES))
	a.put("finegroupactive", checkbox("启用此命名组", 800, 403, 170, 30))
	a.put("finegroupmm", edit("", 800, 460, 130, 34))
	a.put("finegroupsave", button("应用尺寸", 800, 508, 170, 34, ID_FINE_GROUP_SAVE))
	a.put("finesuggest", button("推荐并归组", 250, 552, 165, 30, ID_FINE_AUTO_RECOMMEND))
	a.put("finebulkmm", edit("", 800, 556, 95, 30))
	a.put("finebulkapply", button("整组设置", 906, 556, 110, 30, ID_LEGACY_FINE_BULK_SET))
	send(a.ctl("finecurve"), BM_SETCHECK, 0, 0)
	a.put("export", edit("", 250, 307, 810, 38))
	a.put("exportbutton", button("单独导出 .msh（需确认）", 250, 369, 282, 36, ID_EXPORT_MSH))
	a.put("clearlease", button("解除已停止任务占用", 250, 596, 265, 35, ID_CLEAR_LEASE))
	a.put("tasklog", button("查看任务与错误详情", 530, 596, 190, 35, ID_LEGACY_TASK_LOG))
	a.put("storagechange", button("更改数据存储目录", 250, 525, 230, 35, ID_STORAGE_CHANGE))
	a.put("htmlreport", button("详细质量报告", 252, 591, 220, 38, ID_HTML_REPORT))
	a.put("diagnose", button("风险抽样诊断", 524, 591, 220, 38, ID_DIAGNOSE))
	a.put("qualitycheck", button("只读质量检查", 250, 570, 216, 33, ID_LEGACY_QUALITY_CHECK))
	a.put("mark", button("标记失败单元", 776, 591, 220, 38, ID_MARK))
	a.put("markwarning", button("按默认警告标记", 1005, 580, 195, 38, ID_MARK_WARNING))
	a.put("skewlow", edit("0.90", 360, 657, 72, 30))
	a.put("skewhigh", edit("1.00", 467, 657, 72, 30))
	a.put("markhighskew", button("标记所选偏度", 565, 657, 155, 30, ID_MARK_HIGH_SKEW))
	a.put("warningplan", button("生成局部方案", 250, 624, 172, 34, ID_WARNING_PLAN))
	a.put("warningview", button("查看影响范围", 432, 624, 160, 34, ID_WARNING_VIEW))
	a.put("warningpublish", button("审核发布 Sizing", 602, 612, 205, 34, ID_WARNING_PUBLISH))
	a.put("markrule", combo(250, 483, 220, 34))
	add(a.ctl("markrule"), "ANSYS Worksheet 失败单元")
	add(a.ctl("markrule"), "ANSYS Worksheet 警告单元")
	add(a.ctl("markrule"), "自定义偏度区间")
	add(a.ctl("markrule"), "自定义 Element Quality 区间")
	send(a.ctl("markrule"), CB_SETCURSEL, 0, 0)
	a.put("markexecute", button("标记目标单元", 485, 483, 160, 34, ID_MARK_EXECUTE))
	// Native, scrollable ListView with per-row checkboxes; no webview, browser
	// download or extra ANSYS process required for 200+ candidates.
	a.initializeCandidateList()
	a.put("reviewselectall", button("全选可用", 250, 267, 130, 32, ID_REVIEW_SELECT_ALL))
	a.put("reviewselectsevere", button("偏度 ≥0.95", 390, 267, 140, 32, ID_REVIEW_SELECT_SEVERE))
	a.put("reviewselectwarning", button("0.90–0.95", 540, 267, 140, 32, ID_REVIEW_SELECT_WARNING))
	a.put("reviewselectother", button("其他 / 未取得", 690, 267, 155, 32, ID_REVIEW_SELECT_OTHER))
	a.put("reviewclear", button("清空勾选", 855, 267, 110, 32, ID_REVIEW_CLEAR))
	a.put("reviewrisk", checkbox("我已核查所选区域的几何位置、晶圆间隙和现有 Sizing", 250, 626, 720, 26))
	a.put("reviewback", button("返回质量报告", 250, 652, 170, 34, ID_REVIEW_BACK))
	a.put("reviewdetails", button("查看全部影响域", 462, 652, 180, 34, ID_REVIEW_DETAILS))
	a.put("reviewreason", button("不能发布？", 660, 652, 170, 34, ID_REVIEW_WHY_DISABLED))
	a.put("reviewconfirm", button("审核所选并发布", 870, 652, 220, 34, ID_REVIEW_CONFIRM))
	enabled(a.ctl("reviewconfirm"), false)
	a.put("taskstatus", button("任务状态", 810, 612, 190, 34, ID_LEGACY_TASK_STATUS))
	a.put("analysisbase", combo(250, 205, 300, 34))
	a.put("analysistarget", combo(570, 205, 300, 34))
	a.put("analysisrefresh", button("刷新历史", 885, 205, 112, 34, ID_ANALYSIS_REFRESH))
	a.put("analysisexport", button("导出 CSV", 1007, 205, 112, 34, ID_ANALYSIS_EXPORT))
	a.put("analysisdeep", button("采集原生分布", 1129, 205, 150, 34, ID_ANALYSIS_DEEP_QUALITY))
	a.put("analysisreport", button("导出完整报告", 940, 150, 150, 32, ID_ANALYSIS_REPORT))
	a.put("analysisbodylist", control("EDIT", "", WS_BORDER|WS_TABSTOP|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY, 250, 690, 700, 210, 0))
	font(a.ctl("analysisbodylist"), 12, 400)
	send(a.ctl("analysisbodylist"), 0x00C5 /*EM_LIMITTEXT*/, 2*1024*1024, 0)
	a.put("actionhelp", button("操作说明", 0, 0, 92, 34, ID_ACTION_HELP))
	for _, spec := range []struct {
		key, label string
		id         int
	}{
		{"analysisoverview", "总览与趋势", ID_ANALYSIS_TAB_OVERVIEW},
		{"analysiscompare", "前后对比", ID_ANALYSIS_TAB_COMPARE},
		{"analysisdistribution", "分布分析", ID_ANALYSIS_TAB_DISTRIBUTION},
		{"analysisregion", "区域与历史", ID_ANALYSIS_TAB_REGION},
	} {
		a.put(spec.key, button(spec.label, 250, 150, 150, 32, spec.id))
	}
	// Native right-side task drawer. It is an ordinary local status view; it
	// never dispatches or retries an ANSYS command by opening the panel.
	drawer := control("EDIT", "", WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_READONLY|ES_AUTOVSCROLL|WS_VSCROLL, 0, 0, 10, 10, 0)
	a.put("taskdrawertext", drawer)
	font(drawer, 13, 450)
	a.put("taskdrawerclose", button("关闭", 0, 0, 100, 32, ID_TASK_DRAWER_CLOSE))
	a.put("taskdrawerlog", button("打开任务日志", 0, 0, 140, 32, ID_TASK_DRAWER_LOG))
	if b, err := os.ReadFile(a.ws.path("skew_range_settings.json")); err == nil {
		var saved struct {
			Low  float64 `json:"low"`
			High float64 `json:"high"`
		}
		if json.Unmarshal(b, &saved) == nil && saved.Low >= 0 && saved.High <= 1 && saved.Low < saved.High {
			set(a.ctl("skewlow"), fmt.Sprintf("%.3f", saved.Low))
			set(a.ctl("skewhigh"), fmt.Sprintf("%.3f", saved.High))
		}
	}
	a.lastMarkRule = 0
	a.skewRangeDraft = [2]string{read(a.ctl("skewlow")), read(a.ctl("skewhigh"))}
	a.eqRangeDraft = [2]string{"0.00", "0.10"}
	a.put("taskstop", button("请求停止任务", 20, 690, 165, 30, ID_TASK_STOP))
	visible(a.ctl("taskstop"), false)
	a.switchPage(0)
}
func (a *state) onMarkerRuleChanged() {
	next := int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0))
	previous := a.lastMarkRule
	if previous == 2 {
		a.skewRangeDraft = [2]string{read(a.ctl("skewlow")), read(a.ctl("skewhigh"))}
	} else if previous == 3 {
		a.eqRangeDraft = [2]string{read(a.ctl("skewlow")), read(a.ctl("skewhigh"))}
	}
	if next == 2 && previous != 2 {
		set(a.ctl("skewlow"), a.skewRangeDraft[0])
		set(a.ctl("skewhigh"), a.skewRangeDraft[1])
	} else if next == 3 && previous != 3 {
		set(a.ctl("skewlow"), a.eqRangeDraft[0])
		set(a.ctl("skewhigh"), a.eqRangeDraft[1])
	}
	a.lastMarkRule = next
	custom := a.page == 3 && isCustomQualityRule(next)
	visible(a.ctl("skewlow"), custom)
	visible(a.ctl("skewhigh"), custom)
	a.updateActionStates()
}

func (a *state) switchPage(page int) {
	begin := time.Now()
	a.page = page
	a.scrollY = 0 // Every page opens at its first step, not a stale scrolled position.
	a.scrollX = 0
	if page == 4 {
		_ = a.loadQualityHistory()
	}

	// Page ownership is defined once in pageControlPage. Global controls are
	// handled separately below; navigation never starts a scan or ANSYS task.
	for name, hwnd := range a.controls {
		ownerPage, pageOwned := pageControlPage(name)
		if pageOwned {
			showOnPage := page == ownerPage
			if name == "skewlow" || name == "skewhigh" {
				showOnPage = showOnPage && isCustomQualityRule(int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0)))
			}
			visible(hwnd, showOnPage)
		}
	}
	for _, name := range deprecatedDuplicateControls() {
		visible(a.ctl(name), false)
	}
	for _, name := range []string{"syncstatus", "qualityupdate", "taskcenter", "actionhelp"} {
		visible(a.ctl(name), true)
	}
	visible(a.ctl("analysisbodylist"), page == 4 && a.analysisTab == 3)
	a.syncTaskDrawerControls()
	a.layout() // Reposition child HWNDs when a new page resets the scroll offset.
	a.updateActionStates()
	if elapsed := time.Since(begin); elapsed > navigationDurationMax {
		navigationDurationMax = elapsed
	}
	refresh()
}

// Responsive control positions use the same card geometry as painted layout.
// MoveWindow(repaint=false) followed by one parent invalidation prevents
// flicker and CPU spikes on repeated resize messages.
func (a *state) layout() {
	begin := time.Now()
	if a.ctl("navhome") == 0 || a.layoutInProgress {
		return
	}
	// Native scrollbar visibility may re-enter WM_SIZE on Windows. Never
	// recursively reposition child HWNDs from a nested layout message.
	a.layoutInProgress = true
	defer func() { a.layoutInProgress = false }()
	rc := rect{}
	call(getrect, a.window, uintptr(unsafe.Pointer(&rc)))
	a.scrollY = clampScroll(a.scrollY, a.page, int(rc.B))
	a.scrollX = clampScrollX(a.scrollX, int(rc.R))
	// Native scrollbar exposes the document's overflow; sidebar/header remain fixed.
	info := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: SIF_ALL,
		Min: 0, Max: int32(pageContentBottom(a.page) - 62 + 12 - 1),
		Page: uint32(maxInt(1, int(rc.B)-62)), Pos: int32(a.scrollY)}
	call(setScrollInfo, a.window, SB_VERT, uintptr(unsafe.Pointer(&info)), 1)
	hinfo := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: SIF_ALL,
		Min: 0, Max: int32(uiWidth(int(rc.R)) + 20 - 1),
		Page: uint32(maxInt(1, int(rc.R)-contentX)), Pos: int32(a.scrollX)}
	call(setScrollInfo, a.window, SB_HORZ, uintptr(unsafe.Pointer(&hinfo)), 1)
	for name, box := range plannedControls(int(rc.R)) {
		if strings.HasPrefix(name, "_") {
			continue
		}
		h := a.ctl(name)
		if h == 0 {
			continue
		}
		isSidebar := strings.HasPrefix(name, "nav")
		isTopPinned := name == "syncstatus" || name == "qualityupdate" || name == "taskcenter" || name == "actionhelp"
		y := box.Y
		x := box.X
		if !isSidebar && !isTopPinned {
			y -= a.scrollY
			x -= a.scrollX
		}
		// Retain the native ComboBox dropdown height during every layout/resize.
		// Planned rectangles describe the CLOSED control for overlap tests.
		windowH := nativeComboWindowHeight(name, box.H)
		repoint(h, x, y, box.W, windowH)
		if name == "actionhelp" {
			visible(h, true)
		}
		if !isSidebar && !isTopPinned {
			clipScrolledControl(h, x, y, box.W, windowH, int(rc.R), int(rc.B))
		}
	}
	if a.taskCenterOpen {
		panelW := minInt(440, maxInt(360, int(rc.R)-220))
		panelX := int(rc.R) - panelW
		repoint(a.ctl("taskdrawertext"), panelX+12, 112, panelW-24, maxInt(220, int(rc.B)-224))
		repoint(a.ctl("taskdrawerclose"), int(rc.R)-112, 72, 96, 32)
		if a.busy || a.hasPendingStopTarget() {
			repoint(a.ctl("taskstop"), panelX+18, int(rc.B)-74, 160, 32)
			repoint(a.ctl("taskdrawerlog"), panelX+panelW-170, int(rc.B)-74, 150, 32)
		} else {
			repoint(a.ctl("taskdrawerlog"), panelX+panelW-170, int(rc.B)-74, 150, 32)
		}
	} else {
		repoint(a.ctl("taskstop"), 20, int(rc.B)-98, 165, 30)
	}
	if !resizeActive {
		refresh()
	}
	if elapsed := time.Since(begin); elapsed > layoutDurationMax {
		layoutDurationMax = elapsed
	}
}
func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}
func (a *state) screen(dc uintptr, w, h int) {
	solid(dc, 0, 0, w, h, panelbg, 0)
	solid(dc, 0, 0, w, 62, white, 0)
	rule(dc, 0, 61, w, 61, lineclr)
	solid(dc, 0, 62, 200, h-62, color(251, 252, 255), 0)
	rule(dc, 199, 62, 199, h, lineclr)
	solid(dc, 22, 16, 34, 34, cobalt, 12)
	textAt(dc, "A", 31, 16, 27, 34, 22, 800, white)
	textAt(dc, "AutoMesh", 66, 13, 230, 38, 22, 700, navy)
	if w >= 1080 {
		textAt(dc, "2024 R2  ·  网格工作台", 256, 14, 245, 22, 13, 400, slate)
	} else {
		textAt(dc, "R8.1", 315, 14, 74, 22, 12, 500, slate)
	}
	sharedView := a.sharedDataStatus()
	projectLabel := "未同步"
	if sharedView.Project != "" {
		projectLabel = filepath.Base(strings.TrimRight(sharedView.Project, "/\\"))
	}
	meshLabel := sharedView.MeshSystem
	if meshLabel == "" {
		meshLabel = "未选择"
	}
	qualityLabel := sharedStateLabel(sharedView.QualityState)
	syncLabel := sharedStateLabel(sharedView.EngineeringState)
	geometryLabel := sharedStateLabel(sharedView.GeometryState)
	statusWidth := 255
	if w < 1080 {
		statusWidth = 140
	}
	textAt(dc, shorten("工程 "+shorten(projectLabel, 10)+" · "+shorten(meshLabel, 7)+" · 同步 "+syncLabel+" · 几何 "+geometryLabel+" · 质量 "+qualityLabel, 38), 256, 39, statusWidth, 17, 10, 450, slate)
	switch sharedView.ConnectionState {
	case SharedStateReady:
		solid(dc, w-125, 17, 110, 30, greenbg, 16)
		textAt(dc, "● 已连接", w-115, 18, 90, 27, 12, 600, green)
	case SharedStateLoading:
		solid(dc, w-125, 17, 110, 30, bluepale, 16)
		textAt(dc, "◌ 连接中", w-115, 18, 90, 27, 12, 600, cobalt)
	case SharedStateFailed:
		solid(dc, w-125, 17, 110, 30, stoppale, 16)
		textAt(dc, "● 连接失败", w-115, 18, 90, 27, 12, 600, stopink)
	default:
		solid(dc, w-125, 17, 110, 30, color(241, 244, 249), 16)
		textAt(dc, "○ 未连接", w-115, 18, 90, 27, 12, 500, slate)
	}
	textAt(dc, "工作流程", 23, 77, 150, 25, 12, 650, slate)
	if !a.busy {
		textAt(dc, "下一步", 22, 399, 150, 24, 12, 700, navy)
		next := "选择网格操作"
		if sharedView.ConnectionState != SharedStateReady {
			next = "连接现有 Workbench"
		} else if sharedView.EngineeringState != SharedStateReady {
			next = "同步工程信息"
		} else if sharedView.GeometryState != SharedStateReady {
			next = "执行完整几何扫描"
		} else if sharedView.QualityState != SharedStateReady {
			next = "更新当前网格质量"
		} else if a.page == 3 {
			next = "标记 → 预览 → 审核"
		} else {
			next = "核对设置与质量"
		}
		textAt(dc, next, 22, 427, 163, 23, 11, 450, slate)
		if a.errorText != "" {
			textAt(dc, "有错误记录 · 详情见任务中心", 22, 456, 165, 24, 10, 600, warningcolor)
		}
	}
	// Always-visible progress, including when the user is on page 2 or 3.
	if a.busy {
		textAt(dc, "任务执行中", 20, h-213, 165, 20, 12, 700, cobalt)
		label := "计算中 · 无准确百分比"
		if a.progressPercent >= 0 {
			label = fmt.Sprintf("阶段进度 %d%%", a.progressPercent)
		}
		textAt(dc, label, 20, h-191, 165, 20, 11, 600, navy)
		solid(dc, 20, h-165, 160, 8, color(229, 235, 245), 4)
		if a.progressPercent >= 0 {
			solid(dc, 20, h-165, 160*a.progressPercent/100, 8, cobalt, 4)
		} else {
			// Deterministic activity marker, not a fictional completion rate.
			solid(dc, 20, h-165, 24, 8, cobalt, 4)
		}
		textAt(dc, shorten(a.progress, 21), 20, h-149, 165, 21, 11, 600, slate)
		textAt(dc, shorten(a.details, 23), 20, h-128, 165, 21, 10, 400, slate)
	}
	textAt(dc, "仅接管现有会话", 23, h-58, 168, 19, 11, 450, slate)
	textAt(dc, "不新建工程 · 不运行 Fluent", 23, h-35, 174, 18, 10, 400, slate)
	if !a.busy {
		textAt(dc, "F1 · 操作禁用原因", 22, 486, 170, 21, 10, 500, slate)
	}
	x := 220
	cw := w - 240
	cw = uiWidth(w)
	// A child HWND is repositioned to the same virtual Y as painted content.
	// Clip first in device coordinates, then shift only the content pane.
	saved := call(saveDC, dc)
	call(intersectClip, dc, 200, 62, uintptr(w), uintptr(h))
	call(setViewOrigin, dc, uintptr(int64(-a.scrollX)), uintptr(int64(-a.scrollY)), 0)
	switch a.page {
	case 0:
		a.drawHome(dc, x, cw, w, h)
	case 1:
		a.drawConnect(dc, x, cw)
	case 2:
		a.drawFine(dc, x, cw)
	case 3:
		a.drawReport(dc, x, cw)
	case 4:
		a.drawAnalysis(dc, x, cw)
	case 5:
		a.drawSettings(dc, x, cw)
	case 6:
		a.drawCandidateReview(dc, x, cw)
	}
	if saved != 0 {
		call(restoreDC, dc, saved)
	}
}
func (a *state) drawHome(dc uintptr, x, cw, w, h int) {
	sharedView := a.sharedDataStatus()
	solid(dc, x, 80, cw, 150, color(239, 246, 255), 20)
	textAt(dc, "AutoMesh · 网格优化工作台", x+28, 93, cw-55, 39, 25, 740, navy)
	textAt(dc, "同步工程  →  完整扫描  →  质量检测  →  诊断细化  →  历史对比", x+28, 132, cw-60, 27, 14, 450, slate)
	textAt(dc, "页面切换不触发扫描；质量检测由顶部工具栏统一触发。", x+360, 178, cw-385, 37, 12, 450, slate)
	gap := 12
	col := (cw - 2*gap) / 3
	y := homeSummaryY
	for i := 0; i < 3; i++ {
		solid(dc, x+i*(col+gap), y, col, 179, white, 18)
	}
	textAt(dc, "当前工程", x+20, y+13, col-36, 27, 16, 700, navy)
	rule(dc, x+20, y+45, x+col-20, y+45, lineclr)
	project := "尚未同步"
	if sharedView.Project != "" {
		project = filepath.Base(strings.TrimRight(sharedView.Project, "/\\"))
	}
	textAt(dc, shorten(project, 26), x+20, y+54, col-35, 26, 15, 650, navy)
	sys := sharedView.MeshSystem
	if sys == "" {
		sys = "尚未连接"
	}
	textAt(dc, "网格系统  "+shorten(sys, 23), x+20, y+88, col-35, 22, 13, 450, slate)
	unit := "等待完整几何扫描"
	switch sharedView.GeometryState {
	case SharedStateLoading:
		unit = "完整几何扫描中"
	case SharedStateFailed:
		unit = "完整几何扫描失败"
	case SharedStateStale:
		unit = "上次扫描数据已过期"
	case SharedStateReady:
		if a.snapshot != nil {
			cad := a.snapshot.GeometryUnit
			display := a.snapshot.DisplayLengthUnit
			if display == "" || display == "unknown" {
				display = "未识别"
			}
			unit = "Meshing " + display + " / CAD " + cad
		}
	}
	textAt(dc, "几何规模  "+a.bodyFace(), x+20, y+115, col-35, 24, 13, 450, slate)
	textAt(dc, "几何单位  "+unit, x+20, y+142, col-35, 22, 12, 450, slate)
	x2 := x + col + gap
	textAt(dc, "工程信息同步", x2+20, y+13, col-36, 27, 16, 700, navy)
	rule(dc, x2+20, y+45, x2+col-20, y+45, lineclr)
	if len(a.engineeringSummary) == 0 {
		textAt(dc, sharedView.EngineeringMessage, x2+20, y+60, col-35, 27, 14, 500, slate)
	} else {
		textAt(dc, "几何体  "+printable(a.engineeringSummary["body_count"]), x2+20, y+54, col-35, 25, 13, 500, navy)
		textAt(dc, "几何面  "+printable(a.engineeringSummary["face_count"]), x2+20, y+90, col-35, 25, 13, 500, navy)
		textAt(dc, "命名 / 控制  "+printable(a.engineeringSummary["named_count"])+" / "+printable(a.engineeringSummary["control_count"]), x2+20, y+127, col-35, 25, 13, 500, navy)
		textAt(dc, "状态  "+sharedView.EngineeringMessage, x2+20, y+153, col-35, 18, 10, 450, slate)
	}
	x3 := x + 2*(col+gap)
	textAt(dc, "网格质量", x3+20, y+13, col-36, 27, 16, 700, navy)
	rule(dc, x3+20, y+45, x3+col-20, y+45, lineclr)
	quality := a.qualitySummary
	if quality == nil {
		textAt(dc, "质量数据："+sharedView.QualityMessage, x3+20, y+58, col-40, 24, 13, 550, slate)
	} else {
		q := asMap(quality["quality"])
		skew := asMap(q["skewness"])
		oq := asMap(q["orthogonal_quality"])
		eq := asMap(q["element_quality"])
		textAt(dc, "单元数  "+printable(quality["mesh_elements"]), x3+20, y+55, col-40, 23, 13, 550, navy)
		textAt(dc, "最大偏度  "+printable(skew["max"]), x3+20, y+87, col-40, 23, 13, 550, navy)
		textAt(dc, "正交最小  "+printable(oq["min"])+"  ·  单元质量平均  "+metricText(numberAt(eq, "mean")), x3+20, y+119, col-40, 23, 11, 550, navy)
	}
	qualityState := "质量状态  " + sharedView.QualityMessage
	qualityColor := warningcolor
	if sharedView.QualityState == SharedStateReady {
		qualityColor = green
	} else if sharedView.QualityState == SharedStateFailed {
		qualityColor = stopink
	}
	textAt(dc, qualityState, x3+20, y+151, col-40, 18, 11, 650, qualityColor)
	y2 := homeLowerY
	left := cw * 63 / 100
	right := cw - left - gap
	solid(dc, x, y2, left, 192, white, 18)
	solid(dc, x+left+gap, y2, right, 192, white, 18)
	textAt(dc, "任务状态", x+20, y2+10, left-36, 26, 16, 700, navy)
	rule(dc, x+20, y2+44, x+left-20, y2+44, lineclr)
	if a.busy {
		textAt(dc, "正在执行…", x+left-130, y2+10, 120, 25, 12, 600, cobalt)
	}
	current := a.progress
	if a.progressPercent >= 0 && (a.busy || a.progressPercent == 100) {
		current += fmt.Sprintf(" · %d%%", a.progressPercent)
	}
	textAt(dc, shorten(current, 56), x+20, y2+54, left-45, 27, 14, 600, navy)
	detail := a.status
	if a.busy {
		detail = a.details
	}
	textAt(dc, shorten(detail, 68), x+20, y2+86, left-45, 24, 12, 400, slate)
	if a.busy {
		solid(dc, x+20, y2+123, left-40, 9, color(231, 237, 247), 4)
		if a.progressPercent >= 0 {
			solid(dc, x+20, y2+123, (left-40)*a.progressPercent/100, 9, cobalt, 4)
		}
		textAt(dc, "阶段百分比仅表示可计量步骤；GenerateMesh 等待阶段不估算。", x+20, y2+142, left-40, 27, 11, 400, slate)
	}
	x4 := x + left + gap
	textAt(dc, "网格质量", x4+20, y2+10, right-36, 26, 16, 700, navy)
	rule(dc, x4+20, y2+44, x4+right-20, y2+44, lineclr)
	qualityMetrics := asMap(a.qualitySummary["quality"])
	skew := asMap(qualityMetrics["skewness"])
	textAt(dc, "单元数  "+printable(a.qualitySummary["mesh_elements"]), x4+20, y2+52, right-37, 24, 13, 550, navy)
	textAt(dc, "最大偏度  "+printable(skew["max"]), x4+20, y2+80, right-37, 24, 13, 550, navy)
	gate := sharedView.QualityMessage
	if sharedView.QualityState == SharedStateReady && a.qualitySummary != nil {
		gate = printable(a.qualitySummary["quality_status"]) + " · " + sharedView.QualityMessage
	}
	textAt(dc, "质量状态  "+shorten(gate, 19), x4+20, y2+108, right-37, 24, 12, 500, warningcolor)
	eq := asMap(qualityMetrics["element_quality"])
	textAt(dc, "平均单元质量  "+metricText(numberAt(eq, "mean"))+"（未取得时不估算）", x4+20, y2+139, right-37, 22, 11, 450, slate)
	textAt(dc, "下一步：在局部细化完成完整几何扫描；在质量分析查看历史对比。", x, 645, cw, 22, 11, 400, slate)
}
func (a *state) bodyFace() string {
	view := a.sharedDataStatus()
	if a.snapshot != nil && view.GeometryState == SharedStateReady {
		return fmt.Sprintf("%d 体 / %d 面", len(a.snapshot.Bodies), len(a.snapshot.Faces))
	}
	if view.GeometryState == SharedStateLoading || view.GeometryState == SharedStateFailed || view.GeometryState == SharedStateStale {
		return view.GeometryMessage
	}
	if a.engineeringSummary != nil {
		return printable(a.engineeringSummary["body_count"]) + " 体 / " + printable(a.engineeringSummary["face_count"]) + " 面（轻量同步摘要；未替代完整扫描）"
	}
	return view.GeometryMessage
}
func (a *state) drawConnect(dc uintptr, x, cw int) {
	textAt(dc, "连接工程", x+16, 83, cw-32, 43, 25, 700, navy)
	textAt(dc, "只连接你已经打开的 Workbench，不会新建工程。", x+16, 130, cw-32, 26, 13, 400, slate)
	solid(dc, x, connectCardY, cw, 481, white, 20)
	textAt(dc, "第一步 · 连接 Workbench", x+30, 199, cw-60, 32, 18, 700, navy)
	textAt(dc, "优先复用现有脚本服务，必要时在当前 Workbench 执行 StartServer()。", x+30, 238, cw-60, 27, 13, 400, slate)
	textAt(dc, "手动端口（可选）", x+284, 273, 245, 26, 12, 450, slate)
	rule(dc, x+30, 358, x+cw-30, 358, lineclr)
	textAt(dc, "第二步 · 选择网格系统", x+30, 376, cw-60, 30, 18, 700, navy)
	textAt(dc, "系统名称", x+30, 414, cw-60, 22, 13, 450, slate)
	textAt(dc, "勾选后才能执行扫描；不会自动启动隐藏的 Meshing。", x+30, 544, cw-60, 26, 12, 400, slate)
	rule(dc, x+30, 581, x+cw-30, 581, lineclr)
	connectionView := a.sharedDataStatus()
	status := "待连接：请选择自动查找连接，或使用高级端口设置"
	switch connectionView.ConnectionState {
	case SharedStateLoading:
		if a.initRunning {
			status = "初始化中：检查内部 Worker / 连接脚本（未操作 ANSYS）"
		} else {
			status = "正在连接，请勿重复点击"
		}
	case SharedStateFailed:
		message := a.shared.connectionError
		if message == "" {
			message = a.initError
		}
		status = "连接/初始化失败：" + shorten(message, 85)
	case SharedStateReady:
		status = "Workbench 已连接 · 请选择当前工程中的 Mesh 系统"
	default:
		if !a.ready {
			status = "初始化状态未知：请点击『重新检查初始化』"
		}
	}
	textAt(dc, status, x+30, 596, cw-60, 27, 13, 600, func() uintptr {
		if a.connected {
			return green
		}
		return slate
	}())
	textAt(dc, "只支持本机连接；专业连接参数已在内部配置。", x+30, 631, cw-60, 23, 11, 450, slate)
}

// Local sizing is a proposal, never proof that the candidate really belongs to a wafer.
func (a *state) proposed(name string) (float64, int, string, bool) {
	minSize, count, groupCount := 0.0, 0, 0
	confidence := "medium"
	for _, p := range a.proposals {
		if printable(p["category"]) != name {
			continue
		}
		size, _ := p["size_mm"].(float64)
		n, _ := p["face_count"].(float64)
		if groupCount == 0 || size < minSize {
			minSize = size
		}
		count += int(n)
		groupCount++
		if printable(p["confidence"]) == "named" {
			confidence = "named"
		}
	}
	if groupCount > 1 {
		confidence = fmt.Sprintf("%s / 分组%d", confidence, groupCount)
	}
	return minSize, count, confidence, groupCount > 0
}
func (a *state) boundaryStatus() string {
	if a.snapshot == nil {
		return "未扫描"
	}
	found := map[string]int{"inlet": 0, "outlet": 0}
	for _, n := range a.snapshot.Named {
		k := strings.ToLower(strings.TrimSpace(n.Name))
		for _, prefix := range []string{"inlet", "outlet"} {
			if k == prefix || (strings.HasPrefix(k, prefix) && len(k) > len(prefix) && ((k[len(prefix)] >= '0' && k[len(prefix)] <= '9') || k[len(prefix)] == '_' || k[len(prefix)] == '-')) {
				found[prefix] += len(n.IDs)
			}
		}
	}
	return fmt.Sprintf("命名边界：inlet %d 面 · outlet %d 面（只读验证）", found["inlet"], found["outlet"])
}
func (a *state) drawFine(dc uintptr, x, cw int) {
	textAt(dc, "直接使用 Meshing 现有命名", x+16, 83, cw-32, 43, 25, 700, navy)
	textAt(dc, "不重新选面：读取已有 Named Selection → 按组设置尺寸 → 绑定 Sizing。", x+16, 132, cw-32, 30, 13, 400, slate)
	solid(dc, x, 184, cw, 494, white, 20)
	textAt(dc, "参考尺寸 (mm)", x+30, 162, 100, 24, 12, 550, slate)
	textAt(dc, "默认只作局部尺寸参考；全局覆盖需人工勾选并校核。", x+610, 162, cw-640, 24, 11, 450, slate)
	textAt(dc, "尺寸相近的命名组（单击选择／双击折叠）", x+30, 202, 590, 27, 17, 700, navy)
	textAt(dc, "自动推荐只是待审核方案；不重新选面，不会立即生成网格。", x+30, 231, 550, 23, 12, 450, slate)
	_, rightX, rightW := fineColumns(x + cw + 20)
	textAt(dc, "选中的组或命名", rightX, 216, rightW, 25, 16, 700, navy)
	idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
	selected := "未选择"
	count := ""
	if idx >= 0 && idx < len(a.namedVisible) {
		n := a.namedVisible[idx]
		if groupIndex, ok := parseBucketMarker(n.Name); ok && groupIndex < len(a.sizeBuckets) {
			selected = a.sizeBuckets[groupIndex].Label
			count = fmt.Sprintf("%d 个命名组 · 整组调整", len(a.sizeBuckets[groupIndex].Members))
		} else {
			selected = n.Name
			count = fmt.Sprintf("%d 个几何面 · %s", len(n.IDs), chineseNameHint(n.Name))
		}
	}
	textAt(dc, shorten(selected, 32), rightX, 249, rightW, 43, 14, 650, cobalt)
	textAt(dc, count, rightX, 299, rightW, 24, 13, 450, slate)
	if idx >= 0 && idx < len(a.namedVisible) {
		n := a.namedVisible[idx]
		if _, isGroup := parseBucketMarker(n.Name); !isGroup {
			status := namedSizingCoverage(a.snapshot, n)
			color := slate
			if strings.Contains(status, "交叠") || strings.Contains(status, "不明") || strings.Contains(status, "失效") {
				color = warningcolor
			}
			textAt(dc, shorten("Sizing 覆盖："+status, 47), rightX, 319, rightW, 19, 11, 500, color)
		}
	}
	rule(dc, rightX, 338, x+cw-30, 338, lineclr)
	textAt(dc, "1. 启用／停用选中组或单项", rightX, 355, rightW, 24, 13, 500, navy)
	textAt(dc, "2. 目标尺寸（mm）", rightX, 434, rightW, 24, 13, 500, navy)
	textAt(dc, "应用范围随左侧选择自动切换：单项或整组。", rightX, 537, rightW, 18, 11, 450, navy)
	textAt(dc, "已启用", x+215, 554, 75, 25, 13, 600, navy)
	listW, _, _ := fineColumns(x + cw + 20)
	textAt(dc, fmt.Sprintf("%d 项", len(a.selectedNamed)), x+280, 554, listW-285, 25, 13, 450, green)
	textAt(dc, "操作顺序：核对全局参数 → 可选命名 → 发布 Sizing → 审核并生成网格。", x+30, 591, cw-60, 16, 11, 450, slate)
	proxy, overlap, top := previewSurfaceBudget(a.snapshot, a.selectedNamed)
	feedback := fmt.Sprintf("本页细化代理 %.0f/1500000（非网格单元数或实际内存）；重叠面 %d", proxy, overlap)
	if overlap > 0 {
		feedback += "；请在发布前排除重叠命名"
	}
	fg := slate
	if proxy > 1500000 || overlap > 0 {
		fg = warningcolor
	}
	textAt(dc, shorten(feedback, 130), x+30, 609, cw-60, 16, 10, 500, fg)
	if top != "" {
		textAt(dc, "高预算候选："+shorten(top, 38)+"；真实预算含已有手工控制，以发布前 ACT 为准。", x+30, 629, cw-60, 16, 10, 400, slate)
	}
}

func (a *state) drawReport(dc uintptr, x, cw int) {
	textAt(dc, "质量诊断", x+16, 83, cw-32, 43, 25, 700, navy)
	if a.qualityStale && a.qualityResult != nil {
		textAt(dc, "质量数据可能过期", x+cw-230, 88, 202, 26, 12, 650, warningcolor)
	}
	textAt(dc, "发现异常单元、生成局部优化建议；质量测量统一由顶部「更新质量」触发。", x+16, 130, cw-32, 26, 13, 400, slate)
	textAt(dc, "质量诊断  /  目标标记  /  局部方案审核", x+18, 157, cw-36, 22, 12, 450, slate)
	solid(dc, x, 185, cw, 508, white, 18)
	if a.busy && a.activePhase != "" {
		textAt(dc, "运行中："+taskDisplayName(a.activePhase)+"（尚未确认完成）", x+30, 194, cw-60, 24, 12, 650, warningcolor)
	}
	if a.qualitySummary == nil {
		textAt(dc, "尚无质量检测结果", x+30, 222, cw-60, 34, 18, 600, navy)
		textAt(dc, "点击顶部「更新质量」读取当前 Meshing 实测指标。此操作不需要先完整扫描几何。", x+30, 265, cw-60, 28, 13, 450, slate)
	} else {
		q := asMap(a.qualitySummary["quality"])
		skew := asMap(q["skewness"])
		oq := asMap(q["orthogonal_quality"])
		eq := asMap(q["element_quality"])
		textAt(dc, "当前质量数据 · "+taskStateText(a.qualityResult.Status), x+30, 216, cw-60, 28, 16, 650, navy)
		textAt(dc, "单元数："+printable(a.qualitySummary["mesh_elements"])+"    最大偏度："+printable(skew["max"])+"    平均偏度："+printable(skew["mean"]), x+30, 254, cw-60, 25, 13, 500, navy)
		qualitySecondary := "平均单元质量：" + metricText(numberAt(eq, "mean")) + "（越高越好）"
		if oq["min"] != nil {
			qualitySecondary = "最小正交质量：" + printable(oq["min"]) + "    " + qualitySecondary
		}
		textAt(dc, qualitySecondary, x+30, 284, cw-60, 23, 12, 450, slate)
		textAt(dc, "数据来源："+shorten(printable(a.qualitySummary["quality_source"]), 75)+"；Element Quality："+shorten(printable(eq["source"]), 45), x+30, 307, cw-60, 20, 11, 400, slate)
		if a.qualityStale {
			textAt(dc, "该结果可能不再对应当前网格；请重新更新质量数据后再比较。", x+30, 331, cw-60, 23, 12, 600, warningcolor)
		} else {
			textAt(dc, "质量数据已保存为独立任务回执，可在「质量分析」中与历史版本比较。", x+30, 331, cw-60, 23, 12, 450, green)
		}
		if worksheet := asMap(a.qualitySummary["worksheet_diagnostics"]); len(worksheet) > 0 {
			if metrics, ok := worksheet["metrics"].([]any); ok {
				for i, raw := range metrics {
					if i >= 2 {
						break
					}
					m := asMap(raw)
					line := printable(m["name"]) + "："
					if m["failed_count"] != nil && m["warning_count"] != nil {
						line += "失败 " + printable(m["failed_count"]) + " / 警告 " + printable(m["warning_count"])
					} else {
						line += "接口未返回可核验计数"
					}
					textAt(dc, line, x+30, 364+i*23, cw-60, 21, 12, 450, slate)
				}
			}
		}
	}
	rule(dc, x+30, 421, x+cw-30, 421, lineclr)
	textAt(dc, "01  目标单元标记", x+30, 435, cw-60, 24, 15, 650, navy)
	textAt(dc, "选择一种规则，再执行一次标记；底层仍调用对应的 ANSYS 原生接口。", x+30, 461, cw-60, 20, 12, 450, slate)
	if int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0)) >= 2 {
		metricLabel := "偏度范围"
		if int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0)) == 3 {
			metricLabel = "单元质量区间"
		}
		textAt(dc, metricLabel, x+440, 461, 78, 18, 12, 450, slate)
		textAt(dc, "至", x+515, 485, 15, 30, 12, 450, slate)
		if !validSkewRange(read(a.ctl("skewlow")), read(a.ctl("skewhigh"))) {
			textAt(dc, "范围必须满足 0 ≤ 下限 < 上限 ≤ 1", x+440, 518, cw-470, 17, 11, 500, warningcolor)
		} else if a.sharedDataStatus().GeometryState != SharedStateReady {
			textAt(dc, "标记前请在「局部细化」执行当前 Mesh 完整几何扫描", x+440, 518, cw-470, 17, 11, 500, warningcolor)
		}
	}
	textAt(dc, "02  局部优化方案", x+30, 535, cw-60, 25, 15, 650, navy)
	textAt(dc, "生成方案后查看影响范围，再进入候选审核页；发布与生成网格仍是两个独立步骤。", x+30, 562, cw-60, 24, 12, 450, slate)
	rule(dc, x+30, 607, x+cw-30, 607, lineclr)
	textAt(dc, "风险说明：Worksheet 计数沿用 ANSYS 当前阈值；标记结果不代表局部优化已生效。", x+30, 620, cw-60, 24, 11, 450, warningcolor)
}
func (a *state) drawSettings(dc uintptr, x, cw int) {
	textAt(dc, "设置与维护", x+16, 83, cw-32, 43, 25, 700, navy)
	textAt(dc, "管理导出、数据存储和低频高级维护；日常任务状态统一在顶部任务中心查看。", x+16, 130, cw-32, 26, 13, 400, slate)
	solid(dc, x, 183, cw, 242, white, 20)
	textAt(dc, "导出 Fluent 网格（选填）", x+30, 202, cw-60, 31, 18, 650, navy)
	textAt(dc, "请填写英文绝对 .msh 路径；已存在的文件不会被覆盖。", x+30, 255, cw-60, 26, 13, 450, slate)
	textAt(dc, "导出不是 Fluent 质量验证；边界命名和网格质量必须人工复核。", x+342, 372, cw-377, 31, 12, 500, navy)
	solid(dc, x, 444, cw, 211, white, 20)
	textAt(dc, "数据存储", x+30, 454, 230, 28, 17, 650, navy)
	textAt(dc, "当前："+shorten(a.ws.Dir, 36), x+30, 486, cw/2-45, 24, 12, 450, slate)
	textAt(dc, "迁移前逐文件校验；旧目录保留，不覆盖已有目标。", x+30, 558, cw/2-45, 24, 11, 450, slate)
	textAt(dc, "高级维护", x+cw/2+10, 454, cw/2-40, 28, 17, 650, navy)
	textAt(dc, "仅处理未绑定 Sizing 的旧版辅助命名，不改动人工命名。", x+cw/2+10, 486, cw/2-40, 32, 11, 450, slate)
	textAt(dc, "软件信息", x+cw/2+10, 568, 120, 20, 12, 650, slate)
	textAt(dc, "AutoMesh R8.1 RC · 保留连续 Sizing 安全内核", x+cw/2+10, 590, cw/2-40, 20, 11, 500, navy)
	textAt(dc, "Workbench 2024 R2 桥接 · 当前连接状态见顶部", x+cw/2+10, 612, cw/2-40, 18, 10, 400, slate)
	textAt(dc, "异常占用：先核验 Meshing 后台任务已退出，再使用双重确认解除。", x+30, 632, cw/2-45, 18, 10, 450, warningcolor)
}
func (a *state) busySet(v bool) {
	a.mu.Lock()
	a.busy = v
	if v {
		a.stopSignal = make(chan struct{})
		a.stopRequested = false
		a.progressPercent = -1
		a.details = "正在准备任务；尚未收到 Meshing 阶段数据"
	} else {
		a.stopSignal = nil
		a.stopRequested = false
	}
	a.mu.Unlock()
	a.syncTaskDrawerControls()
	a.updateActionStates()
	refresh()
}
func (a *state) currentStopSignal() <-chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopSignal
}
func (a *state) stopWasRequested() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopRequested
}
func (a *state) hasPendingStopTarget() bool {
	t, pending, _ := a.ws.UnconfirmedActiveTask()
	return pending && len(t.JobID) == 32
}
func (a *state) requestStop() {
	a.mu.Lock()
	active := a.busy && a.stopSignal != nil
	a.mu.Unlock()
	if !active {
		task, pending, err := a.ws.UnconfirmedActiveTask()
		if !pending || len(task.JobID) != 32 {
			alert("当前没有可请求停止的 AutoMesh 任务；可检查历史任务状态。")
			return
		}
		if err != nil {
			alert("不能核实旧任务身份：" + err.Error())
			return
		}
		if !confirm("检测到重启前的未确认任务：" + task.Phase + "（" + task.JobID[:8] + "）。\n\n要向这个任务发送协作式停止请求吗？旧版 Worker 可能不支持取消，且当前原生命令可能继续执行；不会解除任务锁或结束 Meshing 进程。") {
			return
		}
		if e := a.ws.RequestTaskCancel(task.JobID, "operator_after_restart"); e != nil {
			alert("停止请求未保存：" + e.Error())
			return
		}
		a.message("已向历史任务发送停止请求；请检查任务状态确认 ACT 是否响应。任务锁仍保留")
		return
	}
	a.mu.Lock()
	if a.stopRequested {
		a.mu.Unlock()
		a.message("已发出停止请求，等待当前阶段完成或返回；不会重复发送")
		return
	}
	a.mu.Unlock()
	if !confirm("请求安全停止当前 AutoMesh 操作？\n\n只在 Meshing 可响应的检查点停止；正在执行的 GenerateMesh 无法保证立即终止。AutoMesh 不会结束 Meshing 进程，也不会擅自解除任务锁。\n\n点击确认后，如远端停止状态未知，界面可结束等待，但禁止新的修改任务。") {
		return
	}
	a.mu.Lock()
	if a.stopSignal != nil && !a.stopRequested {
		a.stopRequested = true
		close(a.stopSignal)
	}
	a.mu.Unlock()
	a.postProgress("I|已发起安全停止请求|将在可取消检查点退出；若远端状态不明则保留任务锁")
}
func runPython(timeout time.Duration, args ...string) ([]byte, error) {
	python, err := choosePythonCandidate()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, python.exe, append(append([]string{}, python.prefix...), args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("Workbench Python 桥接超时（%s）；任务可能仍在 Meshing 中运行，禁止再次提交: %w", timeout, ctx.Err())
	}
	return out, err
}
func parseResponse(output []byte, dst any) error {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if strings.HasPrefix(s, "{") {
			if json.Unmarshal([]byte(s), dst) == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("外部接口没有返回 JSON：%s", shorten(string(output), 250))
}

type response struct {
	Status, Error, Result, Details, Security string
	Systems                                  []struct {
		Name string
		Mesh bool
	}
}

func (a *state) wbRun(mode, system string) (string, error) {
	return a.wbRunLimited(mode, system, 0)
}

// Only connection discovery uses a short deadline. Explicit user commands and
// long-running Meshing tasks retain their original timeout behavior.
func (a *state) wbRunLimited(mode, system string, probeTimeout time.Duration) (string, error) {
	args := []string{a.ws.path("wb_cli.py"), mode, "--port", strconv.Itoa(a.port), "--security", a.security}
	if mode == "dispatch" {
		args = append(args, "--target", system, "--runner", a.ws.path("Run_In_Existing_Meshing.py"))
	}
	timeout := 18 * time.Second
	if mode == "dispatch" {
		timeout = 46 * time.Minute
	} else if probeTimeout > 0 {
		timeout = probeTimeout
	}
	bytes, err := runPython(timeout, args...)
	// The 0.9 build swallowed a missing-Python/launcher error here: an empty
	// stdout was misreported as "no JSON" even when the subprocess never ran.
	// Preserve the actual process error so the user can distinguish a missing
	// Python runtime from a Workbench transport/security failure.
	if len(strings.TrimSpace(string(bytes))) == 0 {
		if err != nil {
			return "", fmt.Errorf("Python 连接程序未启动或未产生输出：%v。请在 Windows PowerShell 中检查 py -3 -V 及 py -3 -c \"import ansys.workbench.core\"；如缺依赖请安装 ansys-workbench-core", err)
		}
		return "", errors.New("Python 连接程序没有输出；请检查 Python 3 与 ansys-workbench-core 是否安装，并检查安全软件是否拦截")
	}
	var r response
	if x := parseResponse(bytes, &r); x != nil {
		if err != nil {
			return "", fmt.Errorf("%v；Python 进程错误：%v", x, err)
		}
		return "", x
	}
	if r.Status != "ok" {
		// Preserve traceback in a local English-path log; message boxes only
		// show the concise error. The user can share this log if needed.
		if r.Details != "" {
			_ = writeAtomic(a.ws.path("last_workbench_error.log"), []byte(r.Details))
		}
		if r.Error != "" {
			return "", fmt.Errorf("Workbench 连接失败：%s", r.Error)
		}
		return "", errors.New("Workbench 连接未成功，外部接口返回了空错误信息")
	}
	if err != nil {
		return "", fmt.Errorf("Workbench 已响应，但 Python 进程异常结束：%v", err)
	}
	if mode == "list" {
		if r.Security == "mtls" || r.Security == "insecure" || r.Security == "wnua" {
			a.security = r.Security // Preserve verified transport for future writes.
		}
		a.mu.Lock()
		a.systems = nil
		for _, s := range r.Systems {
			if s.Mesh {
				a.systems = append(a.systems, s.Name)
			}
		}
		a.mu.Unlock()
	}
	return r.Result, nil
}
func (a *state) taskRun(phase, system string, snap *Snapshot, plan *Plan) (Result, error) {
	stop := a.currentStopSignal()
	select {
	case <-stop:
		return Result{}, errors.New("用户取消：尚未提交 Meshing 任务")
	default:
	}
	path := ""
	if phase == "auto_onepass" || phase == "auto" || phase == "export" {
		path = a.activeExport
	}
	job, err := a.ws.SubmitForSystem(system, phase, snap, plan, path, false, phase == "export")
	if err != nil {
		return Result{}, err
	}
	// The user can request stop between opening the GUI task and writing the job.
	// Keep its durable lease until ACT acknowledges the cooperative request.
	if a.stopWasRequested() {
		if e := a.ws.RequestTaskCancel(job.JobID, "operator_request"); e != nil {
			return Result{}, e
		}
		return Result{JobID: job.JobID, Phase: phase, Status: "unconfirmed"}, fmt.Errorf("已请求停止任务 %s；远端尚未确认终止。保留任务锁，允许只读检查，禁止重复提交", job.JobID[:8])
	}
	// Persist the transition before the first Workbench dispatch call. If the
	// local lifecycle record cannot be saved, do not send the ACT command.
	if err := a.ws.MarkTaskDispatching(job); err != nil {
		_ = a.ws.removeTaskLeaseIfOwned(job.JobID) // No remote dispatch has started.
		return Result{}, fmt.Errorf("无法保存任务投递状态；尚未向 Workbench 发送命令：%w", err)
	}
	a.postProgress("I|正在提交 ANSYS 任务|任务已锁定，将发送至当前 Meshing：" + phase)
	// Workbench SendCommand may BLOCK until the Meshing command ends. Therefore
	// progress must be polled in PARALLEL with the dispatch subprocess, not
	// only in WaitResult (which is entered after dispatch returns).
	stopProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		last := ""
		taskStarted := time.Now()
		lastStage := "Meshing 命令"
		nextHeartbeat := time.Now().Add(8 * time.Second)
		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				progress, e := readJSON[ActProgress](a.ws.path("job_progress_" + job.JobID + ".json"))
				if e == nil && progress.JobID == job.JobID {
					label := displayActProgress(progress)
					if label != last {
						a.postProgress(label)
						last = label
						lastStage = progress.Stage
					}
				}
				if time.Now().After(nextHeartbeat) {
					if last == "" {
						a.postProgress(fmt.Sprintf("I|等待 Meshing 阶段反馈|已等待 %d 秒；命令可能仍在排队或执行，不要重复点击", int(time.Since(taskStarted).Seconds())))
					} else {
						a.postProgress(fmt.Sprintf("I|%s · 等待更新|任务已运行 %d 秒；ANSYS 暂无新阶段数据，不等于已完成", lastStage, int(time.Since(taskStarted).Seconds())))
					}
					nextHeartbeat = time.Now().Add(8 * time.Second)
				}
			}
		}
	}()
	defer close(stopProgress)
	// Workbench may block inside SendCommand for the entire ACT operation.
	// Poll durable receipts/results concurrently. A timeout is UNCERTAIN,
	// NOT permission to clear or resend the task.
	dispatchDone := make(chan error, 1)
	go func() { _, sendErr := a.wbRun("dispatch", system); dispatchDone <- sendErr }()
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	start := time.Now()
	var dispatchErr error
	returned := false
	lastObserved := ""
	for {
		status, terminal := remoteJobObservation(a.ws, job.JobID)
		if status != lastObserved {
			a.postProgress("I|" + formatObservation(status) + "|任务 " + job.JobID[:8] + " · " + phase)
			lastObserved = status
		}
		if terminal != nil {
			if err := a.ws.releaseTaskLease(job.JobID); err != nil {
				a.postProgress("I|任务已结束但未能释放占用|" + err.Error())
			}
			if terminal.Status == "cancelled" {
				return *terminal, fmt.Errorf("Meshing 已在安全检查点确认取消任务 %s", job.JobID[:8])
			}
			if terminal.Status != "ok" {
				return *terminal, fmt.Errorf("Meshing 返回失败：%s", friendlyACTError(terminal.Error))
			}
			return *terminal, nil
		}
		if reason, crashed := a.ws.meshingProcessCrash(job.JobID); crashed {
			a.postProgress("I|Meshing 进程已退出|" + reason)
			return Result{}, fmt.Errorf("%s。AutoMesh 停止等待，但保留任务锁与现场记录；请核实崩溃原因并确认会话已结束后人工恢复，禁止重复提交", reason)
		}
		if returned && dispatchErr != nil && status == "unconfirmed" {
			return Result{}, fmt.Errorf("Workbench 投递发生异常：%v；远端任务是否接收无法确认。保留任务锁，禁止重发", dispatchErr)
		}
		if time.Since(start) >= 100*time.Second && status != "act_running" && status != "act_stalled" {
			return Result{}, unknownDispatchError(job.JobID, status, time.Since(start))
		}
		// Warning refinement is not a mesh-generation task. A silent ACT
		// heartbeat is not proof of a crash; stop the GUI wait, KEEP the lease,
		// and require read-only task inspection before any further submission.
		if (phase == "warning_refine_preview" || phase == "warning_refine_publish") &&
			time.Since(start) >= 150*time.Second && status == "act_stalled" {
			return Result{}, fmt.Errorf("警告区任务 %s 已等待 %d 秒，Meshing ACT 心跳超过 90 秒未更新。界面停止等待，但任务是否仍在执行尚不确定；保留任务锁，不自动重投递。请点击『检查任务状态』，确认远端任务结束后再处理", job.JobID[:8], int(time.Since(start).Seconds()))
		}
		if time.Since(start) >= 46*time.Minute {
			return Result{}, fmt.Errorf("Meshing 已等待 46 分钟仍未返回终态；任务可能正在执行，保留任务锁。查看任务状态后再处理：%s", job.JobID)
		}
		select {
		case <-stop:
			if e := a.ws.RequestTaskCancel(job.JobID, "operator_request"); e != nil {
				return Result{JobID: job.JobID, Phase: phase, Status: "unconfirmed"}, fmt.Errorf("停止请求写入失败：%w", e)
			}
			return Result{JobID: job.JobID, Phase: phase, Status: "unconfirmed"}, fmt.Errorf("已请求停止 %s，AutoMesh 已结束本轮界面等待；Meshing 原生命令可能继续执行，任务锁保留直到远端确认停止。可查看状态与日志", job.JobID[:8])
		case e := <-dispatchDone:
			returned = true
			dispatchErr = e
			// A transport exception after SendCommand may still mean ACT started.
		case <-poll.C:
		}
	}

}
func (a *state) run(kind string) {
	if !a.ready {
		alert("初始化尚未完成，请稍后查看状态提示")
		return
	}
	if a.busy {
		alert("当前任务仍在执行或等待确认。\n" + a.ws.taskStatus())
		return
	}
	if !a.connected || a.port == 0 || len(a.systems) == 0 {
		a.switchPage(1)
		alert("请先点击「连接 Workbench」，连接你已经打开的工程")
		return
	}
	if send(a.ctl("ack"), BM_GETCHECK, 0, 0) != 1 {
		a.switchPage(1)
		alert("请先确认目标 Meshing 编辑器已打开，避免创建隐藏实例。")
		return
	}
	sel := int(send(a.ctl("systems"), CB_GETCURSEL, 0, 0))
	if sel < 0 || sel >= len(a.systems) {
		alert("请选择目标 Mesh 系统")
		return
	}
	base, err := strconv.ParseFloat(strings.TrimSpace(read(a.ctl("size"))), 64)
	// The reference dimension is only part of the sizing/meshing pipeline.
	// Do not block read-only quality inspection, warning marking or preview
	// because the unrelated text field on Home is currently empty or invalid.
	if taskRequiresReferenceSize(kind) && (err != nil || math.IsNaN(base) || math.IsInf(base, 0) || base < 0.5 || base > 500) {
		a.switchPage(2)
		alert("此操作需要有效的参考尺寸（0.5–500 mm）。请在局部细化页修改后重试。")
		return
	}
	if !taskRequiresReferenceSize(kind) && (err != nil || math.IsNaN(base) || math.IsInf(base, 0)) {
		base = 12 // Unused fallback for independent operations; never applied to Meshing.
	}
	if kind == "export" {
		path := strings.TrimSpace(read(a.ctl("export")))
		if path == "" || !strings.EqualFold(filepath.Ext(path), ".msh") || !filepath.IsAbs(path) || !asciiPath(path) {
			a.switchPage(5)
			alert("请填写英文绝对 .msh 导出路径，例如 D:\\Mesh\\result.msh")
			return
		}
		if _, e := os.Stat(path); e == nil {
			alert("导出文件已经存在：请换一个文件名。本软件禁止覆盖现有 .msh")
			return
		}
		if a.snapshot == nil || a.scannedSystem != a.systems[sel] {
			alert("请先在局部细化页完成完整几何扫描并确认已生成网格，再执行导出")
			return
		}
		if !confirm("将从当前 Meshing 单独导出 Fluent .msh。\n程序会重新检查 inlet/outlet 的 Send to Solver 与初步质量；\n当 OQ 等指标不可用时，导出文件仅供 Fluent 再验证，不代表网格合格。\n拒绝覆盖已有文件。确定继续？") {
			return
		}
	}
	if kind == "rename_auxiliary" {
		if a.snapshot == nil || a.scannedSystem != a.systems[sel] {
			alert("请先刷新 Meshing 命名，再执行中文化")
			return
		}
		if !confirm("仅将旧版 AutoMesh 自建、未绑定 Sizing 的 AM_* 辅助命名改成中文。\n不触碰 inlet/outlet、已有人工命名或绑定了 Sizing 的组；完成后需重新扫描。确认继续？") {
			return
		}
	}
	if kind == "mark_failed" || kind == "mark_warning" {
		if !confirm("将调用 ANSYS Meshing Quality Worksheet API，在当前模型按阈值创建失败或警告元素选择组。\n不会清除或重划现有网格；此操作会改变命名选择树。\n\n是否继续？") {
			return
		}
	}
	if kind == "mark_high_skew" || kind == "mark_low_element_quality" {
		if a.snapshot == nil || a.scannedSystem != a.systems[sel] {
			alert("请先在局部细化页完成完整几何扫描，再按真实网格质量区间标记单元。")
			return
		}
		if !confirm("将按自定义质量上下限，在当前网格创建原生 Mesh Element 选择组；不调整原质量警告阈值、不清网格、不发布 Sizing。\n低 Element Quality 可能与几何/网格方法有关，后续 Sizing 必须人工审核。确认继续？") {
			return
		}
	}
	if kind == "warning_refine_preview" {
		if !confirm("仅读取本次由 ANSYS 官方 Worksheet 标记的警告单元，生成球形影响域建议。\n不新建 Sizing、不清理或重新生成网格。确认分析？") {
			return
		}
	}
	if kind == "warning_refine_publish" {
		proposal, proofErr := a.ws.ReadReviewedWarningPlan(a.snapshot)
		if proofErr != nil {
			alert("无法审核发布：" + proofErr.Error() + "\n\n没有成功生成并审核过的警告区方案时，绝不创建 Sizing。")
			return
		}
		selected, selectionErr := a.ws.readWarningSelection(proposal)
		if selectionErr != nil {
			alert("请先手动勾选并审核候选：" + selectionErr.Error())
			return
		}
		msg := fmt.Sprintf("方案 %s：全部候选 %d 处，本次明确勾选 %d 处，预计累计预算代理 %d（建议预警值120000，超限已单独确认）。\n\n将仅为本次所选区域创建可编辑 Sphere of Influence Sizing。每批最多 6 个；其他区域保留待处理。\n\n片间净空尚未自动核验：你必须已在影响域详情逐项核查。\n不会清除或重新生成网格。\n\n确认已保存工程，授权发布这 %d 个区域？", proposal.PlanID, len(proposal.Clusters), len(selected.CandidateIDs), selected.SelectedBudget, len(selected.CandidateIDs))
		if !confirm(msg) {
			return
		}
	}
	if kind == "publish_existing_sizing" {
		if a.snapshot == nil || a.scannedSystem != a.systems[sel] {
			a.switchPage(0)
			alert("先在局部细化页点击「完整几何扫描」读取当前 Meshing 的 Named Selection。")
			return
		}
		if len(a.selectedNamed) > 40 {
			alert("一次最多发布 40 个 Named Selection；请关闭不需要的组，分批设置并在 Meshing 中复核")
			return
		}
		if len(a.selectedNamed) == 0 && !hasVerifiedRetirableControls(a.ws.Dir, a.snapshot) {
			a.switchPage(2)
			alert("没有启用项，且找不到该工程可安全停用的已发布 AutoMesh Sizing。请先选择命名组。")
			return
		}
		if send(a.ctl("global"), BM_GETCHECK, 0, 0) == 1 {
			alert("直接按命名发布时不会覆盖全局尺寸；请先在概览关闭「覆盖全局尺寸」。")
			return
		}
		if !confirm(fmt.Sprintf("同步 %d 个已勾选命名组的 Sizing。\n已取消勾选的、确认为 AutoMesh 所创建的旧 Sizing 将 Suppress（不会删除）；重新勾选会恢复。\n人工控制绝不自动修改；不生成网格。\n请确认已保存工程，继续？", len(a.selectedNamed))) {
			return
		}
	}
	if kind == "prepare" || kind == "publish_names" {
		if a.scannedBaseMM != base {
			a.switchPage(0)
			alert("参考尺寸在扫描后发生变化。请重新执行完整几何扫描，以刷新局部细化建议。")
			return
		}
		if a.snapshot == nil || a.scannedSystem != a.systems[sel] || len(a.proposals) == 0 {
			a.switchPage(0)
			alert("必须先执行当前网格系统的完整几何扫描并审核局部细化方案；本版本不允许绕过审核直接生成。")
			return
		}
		if kind == "publish_names" {
			if !confirm("仅发布 AM_* Named Selection（不创建 Sizing、不清除网格、不调用 GenerateMesh）。\n已有 inlet/outlet 不修改；如 ANSYS 导致网格状态变化，请在 Meshing 检查。确定继续？") {
				return
			}
		} else {
			if !confirm("将在现有 Meshing 中发布 AM_* 命名组及 AutoMesh_* Sizing。\n不会调用 Clear Generated Data 或 GenerateMesh；不会覆盖已有人工控制。确定继续？") {
				return
			}
		}
	}
	if kind == "generate_reviewed" {
		if a.snapshot == nil || a.scannedSystem != a.systems[sel] ||
			(a.preparedSystem != a.systems[sel] && !hasVerifiedManagedManifest(a.ws.Dir, a.snapshot)) {
			alert("请先扫描并发布可编辑的命名组 / Sizing，核对后才能生成网格。")
			return
		}
		mem := freeGiB()
		if mem >= 0 && mem < 9 {
			alert(fmt.Sprintf("当前可用内存仅 %.1f GiB；禁止执行网格生成。", mem))
			return
		}
		if !confirm("生成前审核（以下全局数据来自上一次扫描，真实 ACT 将再次核验）：\n" + globalReviewText(a.snapshot, base, send(a.ctl("global"), BM_GETCHECK, 0, 0) == 1) + "\n\n仅在你确认后才进行受控重划；已有网格若清除则不保证恢复。请先保存工程。\n确认提交审核生成？") {
			return
		}
	}
	system := a.systems[sel]
	var proposal *Plan
	if kind == "mark_high_skew" || kind == "mark_low_element_quality" {
		minV, e1 := strconv.ParseFloat(strings.TrimSpace(read(a.ctl("skewlow"))), 64)
		maxV, e2 := strconv.ParseFloat(strings.TrimSpace(read(a.ctl("skewhigh"))), 64)
		if e1 != nil || e2 != nil || math.IsNaN(minV) || math.IsNaN(maxV) || math.IsInf(minV, 0) || math.IsInf(maxV, 0) || minV < 0 || maxV > 1 || minV >= maxV {
			alert("质量指标范围错误：请输入 0 ≤ 下限 < 上限 ≤ 1")
			return
		}
		if kind == "mark_low_element_quality" {
			proposal = &Plan{QualityMin: minV, QualityMax: maxV}
		} else {
			proposal = &Plan{SkewMin: minV, SkewMax: maxV}
			_ = writeJSON(a.ws.path("skew_range_settings.json"), map[string]float64{"low": minV, "high": maxV})
		}
	}
	if kind == "publish_existing_sizing" {
		raw := strings.TrimSpace(printable(a.snapshot.Mesh["Elements"]))
		n, e := strconv.Atoi(raw)
		if e != nil || n < 0 {
			alert("无法确认现有网格状态，请重新扫描 Meshing。")
			return
		}
		existingMesh := n > 0
		if existingMesh && !confirm("当前工程已有网格，新增局部 Sizing 可能使网格失效。AutoMesh 不会清除或重划；请先保存工程。确认继续？") {
			return
		}
		sizes := make(map[string]float64, len(a.selectedNamed))
		scopes := make(map[string][]int, len(a.selectedNamed))
		for k, v := range a.selectedNamed {
			sizes[k] = v
		}
		for _, n := range a.snapshot.Named {
			if _, wanted := sizes[n.Name]; wanted {
				scopes[n.Name] = append([]int(nil), n.IDs...)
			}
		}
		proposal = &Plan{GlobalMM: base, ExistingNamedSizes: sizes, ExistingNamedScopeIDs: scopes, AllowExistingMeshSizing: existingMesh}
	}
	if kind == "prepare" || kind == "publish_names" {
		existingMesh := false
		if kind == "prepare" {
			raw := strings.TrimSpace(printable(a.snapshot.Mesh["Elements"]))
			n, e := strconv.Atoi(raw)
			if e != nil || n < 0 {
				alert("无法读取当前 Meshing 是否已有网格。请重新只读扫描后再发布 Sizing。")
				return
			}
			existingMesh = n > 0
			if existingMesh {
				if send(a.ctl("global"), BM_GETCHECK, 0, 0) == 1 {
					alert("已有网格时不能覆盖全局尺寸，请在局部细化页关闭该选项。")
					return
				}
				if !confirm("当前 Meshing 已经生成网格。发布局部 Sizing 可能使 ANSYS 现有网格失效。\n软件不会自动清网格，也不会自动重划；请先保存你的工程并确认可以接受风险。\n仍然继续吗？") {
					return
				}
			}
		}
		proposal = &Plan{AllowExistingMeshSizing: existingMesh, GlobalMM: base, ApplyGlobal: send(a.ctl("global"), BM_GETCHECK, 0, 0) == 1,
			LocalOverrides: map[string]float64{}, DisabledRegions: []string{},
			GroupOverrides: map[string]float64{}, DisabledGroups: []string{}}
		for k, v := range a.groupOverrides {
			proposal.GroupOverrides[k] = v
		}
		for k, on := range a.groupDisabled {
			if on {
				proposal.DisabledGroups = append(proposal.DisabledGroups, k)
			}
		}
		for k, on := range a.groupEnabled {
			if on {
				proposal.EnabledGroups = append(proposal.EnabledGroups, k)
			}
		}
		sort.Strings(proposal.DisabledGroups)
		sort.Strings(proposal.EnabledGroups)
		for _, def := range []struct {
			key        string
			categories []string
		}{
			{"gap", []string{"area_small"}}, {"hole", []string{"area_repeated"}}, {"port", []string{"named_existing"}}, {"curve", []string{"area_review"}},
		} {
			if send(a.ctl("fine"+def.key), BM_GETCHECK, 0, 0) != 1 {
				proposal.DisabledRegions = append(proposal.DisabledRegions, def.categories...)
				continue
			}
			input := strings.TrimSpace(read(a.ctl("fine" + def.key + "mm")))
			if input != "" {
				value, e := strconv.ParseFloat(input, 64)
				if e != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0.35 || value > 500 {
					alert("局部细化目标尺寸必须是 0.35–500 mm 的有效数字；留空则采用扫描建议。")
					return
				}
				for _, cat := range def.categories {
					proposal.LocalOverrides[cat] = value
				}
			}
		}
	}
	a.activeExport = strings.TrimSpace(read(a.ctl("export")))
	a.activeApplyGlobal = send(a.ctl("global"), BM_GETCHECK, 0, 0) == 1
	if kind == "mark_high_skew" || kind == "mark_warning" || kind == "warning_refine_preview" {
		// A new preview attempt must not leave an old reviewed plan publishable
		// if ACT returns no candidates or fails partway through.
		if err := a.ws.InvalidateWarningPreview(); err != nil {
			alert("旧优化方案状态无法安全失效：" + err.Error())
			return
		}
	}
	a.activePhase = kind
	a.busySet(true)
	a.postProgress("正在检查当前工程…")
	started := time.Now()
	go func() {
		// Sample system free RAM while the current ANSYS task is active.
		// This is NOT ANSYS process RSS or a GPU/CPU utilization measurement.
		stopRAM := make(chan struct{})
		ramMinimum := make(chan float64, 1)
		startRAM := freeGiB()
		go func() {
			minimum := startRAM
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopRAM:
					ramMinimum <- minimum
					return
				case <-ticker.C:
					available := freeGiB()
					if available >= 0 && (minimum < 0 || available < minimum) {
						minimum = available
					}
				}
			}
		}()
		var result Result
		var err error
		if kind == "sync_status" {
			a.postProgress("I|同步工程信息|仅读取工程、网格规模、命名与现有控制；不扫描完整面几何")
			result, err = a.taskRun("sync_status", system, nil, nil)
		} else if (kind == "quality_check" || kind == "quality_check_deep" || kind == "diagnose") && (a.snapshot == nil || a.scannedSystem != system) {
			a.postProgress("I|读取当前网格质量|无可复用几何快照；仅执行当前网格只读质量读取")
			result, err = a.taskRun(kind, system, nil, nil)
		} else if kind == "export" || kind == "rename_auxiliary" {
			// Both operations already require a verified snapshot at the UI gate.
			// Do not dispatch another full inspect before their native checks.
			result, err = a.taskRun(kind, system, a.snapshot, nil)
		} else if kind == "publish_existing_sizing" {
			a.postProgress("I|绑定现有命名|仅为勾选的 Named Selection 创建 Sizing")
			result, err = a.taskRun("publish_existing_sizing", system, a.snapshot, proposal)
		} else if kind == "prepare" || kind == "publish_names" {
			if kind == "publish_names" {
				a.postProgress("正在仅创建可编辑命名组，不生成网格…")
				result, err = a.taskRun("publish_names", system, a.snapshot, proposal)
			} else {
				a.postProgress("复核几何 → 发布可编辑命名组与局部 Sizing（不生成网格）…")
				result, err = a.taskRun("prepare", system, a.snapshot, proposal)
			}
		} else if kind == "generate_reviewed" {
			a.postProgress("校验已审核命名组和尺寸，准备生成网格…")
			result, err = a.taskRun("generate_reviewed", system, a.snapshot, nil)
		} else if (kind == "quality_check" || kind == "quality_check_deep" || kind == "diagnose" || kind == "mark_failed" || kind == "mark_warning" || kind == "mark_high_skew" || kind == "mark_low_element_quality" || kind == "warning_refine_preview" || kind == "warning_refine_publish") && a.snapshot != nil && a.scannedSystem == system {
			a.postProgress("I|核验当前网格|复用已扫描几何；将由 ACT 再验证会话与模型")
			if kind == "mark_high_skew" || kind == "mark_low_element_quality" {
				result, err = a.taskRun(kind, system, a.snapshot, proposal)
			} else {
				result, err = a.taskRun(kind, system, a.snapshot, nil)
			}
		} else {
			result, err = a.taskRun("inspect", system, nil, &Plan{GlobalMM: base})
			if err == nil && kind != "inspect" {
				snap, e := a.ws.LoadSnapshot()
				if e != nil {
					err = e
				} else {
					// The implicit inspect succeeded even if the next task
					// subsequently fails. Reuse this genuine scan on later pages.
					a.mu.Lock()
					a.snapshot = &snap
					a.scannedSystem = system
					a.scannedBaseMM = base
					a.shared.geometryScannedAt = time.Now()
					a.shared.geometryStale = false
					a.shared.geometryError = ""
					if a.qualityResult != nil && (a.qualityFingerprint != "" && a.qualityFingerprint != snap.Fingerprint ||
						a.qualityMeshSignature != "" && snapshotMeshSignature(&snap) != "" && a.qualityMeshSignature != snapshotMeshSignature(&snap) ||
						a.qualityQuickGeometrySignature != "" && snap.GeometryQuickSignature != "" && a.qualityQuickGeometrySignature != snap.GeometryQuickSignature) {
						a.qualityStale = true
					}
					a.mu.Unlock()
					p := &Plan{GlobalMM: base, ApplyGlobal: a.activeApplyGlobal}
					result, err = a.taskRun(kind, system, &snap, p)
				}
			}
		}
		if kind == "warning_refine_preview" && err == nil {
			if e := a.ws.RecordWarningPreview(result, a.snapshot); e != nil {
				err = fmt.Errorf("Meshing 已返回预览，但诊断文件尚未安全保存：%w", e)
			}
		}
		if err == nil && (kind == "mark_warning" || kind == "mark_high_skew" || kind == "mark_low_element_quality" || kind == "generate_reviewed" || kind == "prepare" || kind == "publish_existing_sizing") {
			// Previous preview might no longer describe the active mesh controls.
			_ = a.ws.InvalidateWarningPreview()
		}
		close(stopRAM)
		minimumFreeRAM := <-ramMinimum
		// Total wall-clock includes ACT scanning, Python/Workbench transport and meshing.
		timing := map[string]any{
			"job_id": result.JobID, "requested_operation": kind,
			"elapsed_seconds": math.Round(time.Since(started).Seconds()*100) / 100,
			"status":          "completed",
		}
		if err != nil {
			timing["status"] = "error"
		}
		if minimumFreeRAM >= 0 {
			timing["min_system_free_ram_gib"] = math.Round(minimumFreeRAM*100) / 100
		}
		_ = writeJSON(a.ws.path("last_task_timing.json"), timing)
		if err == nil {
			if kind == "inspect" {
				a.mu.Lock()
				a.scannedSystem = system
				a.scannedBaseMM = base
				a.mu.Unlock()
			}
			if kind == "publish_names" || kind == "rename_auxiliary" {
				a.mu.Lock()
				a.preparedSystem = ""
				a.scannedSystem = ""
				a.snapshot = nil // newly published names make prior cached scopes stale
				a.shared.geometryStale = true
				a.shared.geometryError = ""
				a.mu.Unlock()
			}
			if kind == "prepare" || kind == "publish_existing_sizing" {
				a.mu.Lock()
				a.preparedSystem = system
				a.mu.Unlock()
			}
			if kind != "publish_names" && kind != "rename_auxiliary" && kind != "sync_status" {
				if snap, e := a.ws.LoadSnapshot(); e == nil {
					a.mu.Lock()
					a.snapshot = &snap
					if kind == "inspect" {
						a.shared.geometryScannedAt = time.Now()
						a.shared.geometryStale = false
						a.shared.geometryError = ""
					}
					if a.qualityResult != nil {
						meshSignature := snapshotMeshSignature(&snap)
						quickGeometry := snap.GeometryQuickSignature
						if a.qualityFingerprint != "" && a.qualityFingerprint != snap.Fingerprint {
							a.qualityStale = true
						} else if a.qualityMeshSignature != "" && meshSignature != "" && a.qualityMeshSignature != meshSignature {
							a.qualityStale = true
						} else if a.qualityQuickGeometrySignature != "" && quickGeometry != "" && a.qualityQuickGeometrySignature != quickGeometry {
							a.qualityStale = true
						}
					}
					a.mu.Unlock()
				} else if kind == "inspect" {
					a.mu.Lock()
					a.scannedSystem = ""
					a.shared.geometryStale = true
					a.shared.geometryError = "完整几何扫描已返回，但本地 Snapshot 读取失败：" + e.Error()
					a.mu.Unlock()
				}
			}
		}
		a.mu.Lock()
		if err != nil {
			a.errorText = err.Error()
		} else {
			a.errorText = ""
		}
		if err != nil {
			switch kind {
			case "sync_status":
				a.shared.engineeringError = err.Error()
				a.shared.engineeringStale = true
			case "inspect":
				a.shared.geometryError = err.Error()
				a.shared.geometryStale = true
			case "quality_check", "quality_check_deep":
				a.shared.qualityError = err.Error()
				a.qualityStale = true
			}
		}
		if err == nil {
			switch kind {
			case "publish_names", "rename_auxiliary", "prepare", "publish_existing_sizing", "warning_refine_publish", "generate_reviewed", "auto_onepass", "auto":
				a.shared.engineeringStale = true
			}
		}
		if result.JobID != "" {
			a.last = &result
			var sm map[string]any
			_ = json.Unmarshal(result.Summary, &sm)
			a.lastSummary = sm
			if result.Phase == "sync_status" {
				a.engineeringSummary = sm
				a.engineeringSyncedAt = time.Now()
				a.shared.engineeringSystem = system
				a.shared.engineeringStale = false
				a.shared.engineeringError = ""
				if a.qualityResult != nil {
					newMesh := printable(sm["mesh_signature"])
					newGeometry := printable(sm["geometry_quick_signature"])
					newProject := printable(sm["project"])
					if (a.qualityMeshSignature != "" && newMesh != "—" && a.qualityMeshSignature != newMesh) ||
						(a.qualityQuickGeometrySignature != "" && newGeometry != "—" && a.qualityQuickGeometrySignature != newGeometry) ||
						(a.qualityProject != "" && newProject != "—" && !strings.EqualFold(filepath.Clean(a.qualityProject), filepath.Clean(newProject))) {
						a.qualityStale = true
					}
				}
			}
			if result.Phase == "quality_check" || result.Phase == "diagnose" || result.Phase == "generate_reviewed" || result.Phase == "auto_onepass" || result.Phase == "auto" {
				if result.Status == "ok" && (len(asMap(sm["quality"])) > 0 ||
					(printable(sm["quality_status"]) == "not_measured" && printable(sm["quality_gate"]) == "no_mesh")) {
					cpy := result
					a.qualityResult = &cpy
					a.qualitySummary = sm
					a.qualityCheckedAt = time.Now()
					a.qualityStale = false
					a.shared.qualitySystem = system
					a.shared.qualityError = ""
					a.qualityFingerprint = printable(sm["geometry_fingerprint"])
					a.qualityMeshSignature = printable(sm["mesh_signature"])
					a.qualityProject = printable(sm["project"])
					a.qualityQuickGeometrySignature = printable(sm["geometry_quick_signature"])
				}
			} else if result.Phase != "inspect" && result.Phase != "sync_status" && result.Phase != "mark_failed" && result.Phase != "mark_warning" && result.Phase != "mark_high_skew" && result.Phase != "mark_low_element_quality" && result.Phase != "warning_refine_preview" && result.Phase != "publish_names" && result.Phase != "rename_auxiliary" {
				a.qualityStale = true
			}
		}
		a.mu.Unlock()
		call(post, a.window, WM_APPDONE, 0, 0)
	}()
}

// Initialization only stages versioned local scripts. It never connects to,
// changes, or launches an ANSYS session. A retry never clears an unknown lease.
func (a *state) beginInitialization() {
	// Read-only recovery. Never clear an unconfirmed task on restart.
	if lease, e := readJSON[Task](a.ws.taskLeasePath()); e == nil && lease.JobID != "" {
		a.message("检测到前次任务 " + lease.Phase + "（" + lease.JobID[:8] + "）；保留占用，需检查远端是否结束")
	}
	a.mu.Lock()
	if a.initRunning {
		a.mu.Unlock()
		a.message("内部脚本检查已经在执行，不重复启动")
		return
	}
	if a.busy || a.connected {
		a.mu.Unlock()
		alert("当前已连接 Workbench 或有 ANSYS 任务执行，禁止重新安装内部脚本。")
		return
	}
	a.initRunning = true
	a.initError = ""
	a.ready = false
	a.initStarted = time.Now()
	a.initFinished = time.Time{}
	a.mu.Unlock()
	a.message("正在校验 AutoMesh 内部脚本；此时不会连接或更改 ANSYS")
	go func() {
		err := a.ws.InitializeScriptsAndRecord()
		a.mu.Lock()
		if err != nil {
			a.initError = err.Error()
		}
		a.mu.Unlock()
		call(post, a.window, WM_APPINIT, 0, 0)
	}()
}

func (a *state) connect(automatic bool) {
	a.mu.Lock()
	ready, running, initErr := a.ready, a.initRunning, a.initError
	a.mu.Unlock()
	if !ready {
		if running {
			a.message("内部脚本正在初始化；尚未开始连接 Workbench")
			return
		}
		if initErr != "" {
			alert("AutoMesh 内部脚本初始化失败，不是 Workbench 连接错误。\n\n原因：" + shorten(initErr, 650) + "\n\n数据目录：" + a.ws.Dir + "\n\n请点击『重新检查初始化』复核。不会清除任务锁或修改 Meshing。")
			return
		}
		alert("初始化尚未确认完成。点击『重新检查初始化』获取详细原因。")
		return
	}
	if a.busy {
		a.postProgress("I|当前有任务运行|连接操作未重复提交；请先检查当前任务状态")
		return
	}
	if task, pending, err := a.ws.UnconfirmedActiveTask(); pending {
		msg := "检测到未确认终止的任务 " + task.Phase + "（" + shorten(task.JobID, 8) + "）。为避免影响仍在运行的 Meshing，不自动启动或重新连接脚本服务。请先只读检查任务状态。"
		if err != nil {
			msg += "；状态文件异常：" + shorten(err.Error(), 100)
		}
		a.message(msg)
		if !automatic {
			alert(msg)
		}
		return
	}
	portText := strings.TrimSpace(read(a.ctl("port")))
	preferred := 10605 // optional hint, not a required manual server port
	var err error
	if portText != "" && portText != "自动" && !strings.EqualFold(portText, "auto") {
		preferred, err = strconv.Atoi(portText)
		if err != nil || preferred < 1 || preferred > 65535 {
			alert("端口可留空或填「自动」；如填写具体端口，必须为 1–65535。")
			return
		}
	}
	a.security = "auto" // Detect actual StartServer transport; never guess insecure.
	a.mu.Lock()
	a.autoConnecting = automatic
	a.mu.Unlock()
	a.busySet(true)
	a.activePhase = "connect"
	a.postProgress("I|查找已有 Workbench 服务|优先验证上次端口；失败后扫描本机可用端口")
	go func() {
		var finalErr error
		verified := false
		seenWorkbenchServer := false
		var autoStartError error
		// IMPORTANT: a Workbench process normally owns several unrelated TCP
		// listeners.  They are NOT evidence that StartServer() is running.
		// R7.2 incorrectly refused to bootstrap whenever ANY WB listener
		// existed and tried up to 32 ports with 18-second timeouts.
		ports, discoveryDetail, discoveryErr := a.discoverExistingServerPorts(preferred)
		if discoveryErr != nil {
			a.postProgress("I|端口清单读取失败|快速检查旧端口，不扫描全部服务: " + shorten(discoveryErr.Error(), 90))
			ports = discoverLocalPorts(preferred, preferred, preferred)
		} else {
			a.postProgress("I|已取得 Workbench 端口清单|" + discoveryDetail)
		}
		customPort := portText != "" && portText != "自动" && !strings.EqualFold(portText, "auto")
		if customPort && hasLocalListener(preferred) {
			ports = uniqueCandidatePorts([]int{preferred}, ports, 8)
		}
		// Bound the initial read-only checks. Never let unrelated services
		// delay launching the already-open Workbench scripting server.
		for i, candidate := range ports {
			if i >= 5 || a.stopWasRequested() {
				break
			}
			a.port = candidate
			a.security = "auto"
			a.postProgress(fmt.Sprintf("I|检查现有脚本服务|端口 %d（%d/%d），快速验证", candidate, i+1, min(len(ports), 5)))
			_, err = a.wbRunLimited("list", "", 9*time.Second)
			if err != nil {
				finalErr = err
				continue
			}
			a.mu.Lock()
			valid := len(a.systems) > 0
			a.mu.Unlock()
			if valid {
				verified = true
			} else {
				seenWorkbenchServer = true
				finalErr = fmt.Errorf("Workbench 服务已验证，但没有 Mesh 系统")
			}
			break
		}
		if !verified && !seenWorkbenchServer && !a.stopWasRequested() {
			// No verified server: START IT even when WB owns ordinary
			// listener ports.  The helper must identify exactly one existing
			// command window and never start a second Workbench process.
			a.postProgress("I|当前没有验证到脚本服务|尝试在已打开的 Workbench 中执行 StartServer()")
			bootstrap, startErr := a.bootstrapExistingWorkbench()
			autoStartError = startErr
			if startErr == nil {
				a.postProgress("I|StartServer 命令已发送|正在确认新的脚本服务端口")
				// When the server starts on a previously used port, a simple
				// socket-difference can be empty. Re-enumerate every WB-owned
				// listener instead of falling back to the old 10000..11200 span.
				fresh, _, discoverErr := a.discoverExistingServerPorts(preferred)
				if discoverErr != nil {
					fresh = nil
				}
				trial := uniqueCandidatePorts(bootstrap.CandidatePorts, fresh, 10)
				for i, candidate := range trial {
					if i >= 8 || a.stopWasRequested() {
						break
					}
					a.port = candidate
					a.security = "auto"
					a.postProgress(fmt.Sprintf("I|核验新启动的服务|端口 %d（%d/%d）", candidate, i+1, min(len(trial), 8)))
					_, err = a.wbRunLimited("list", "", 10*time.Second)
					if err != nil {
						finalErr = err
						continue
					}
					a.mu.Lock()
					valid := len(a.systems) > 0
					a.mu.Unlock()
					if valid {
						verified = true
					} else {
						seenWorkbenchServer = true
						finalErr = fmt.Errorf("Workbench 服务已连接，但没有 Mesh 系统")
					}
					break
				}
				if !verified && finalErr == nil {
					finalErr = fmt.Errorf("StartServer() 已提交，但未验证到可用脚本服务；未自动重发")
				}
			}
		}

		if a.stopWasRequested() {
			a.mu.Lock()
			a.errorText = "连接已按请求停止；未新建或修改 ANSYS 工程"
			a.mu.Unlock()
			call(post, a.window, WM_APPDONE, 1, 0)
			return
		}
		if !verified {
			base := "未能验证当前 Workbench 的网格脚本服务。AutoMesh 已尝试自动识别现有端口；仅在安全识别命令窗口时才自动执行 StartServer()，绝不新建 Workbench。"
			if autoStartError != nil {
				base += "\n自动启动诊断：" + shorten(autoStartError.Error(), 220)
			}
			if finalErr != nil {
				base += "\n连接诊断：" + shorten(finalErr.Error(), 180)
			}
			a.mu.Lock()
			a.errorText = base
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.errorText = ""
			a.mu.Unlock()
		}
		call(post, a.window, WM_APPDONE, 1, 0)
	}()
}

func hasLocalListener(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	l := discoverLocalPorts(port, port, port)
	return len(l) > 0
}

func (a *state) finish(connecting bool) {
	a.mu.Lock()
	e := a.errorText
	a.mu.Unlock()
	a.busySet(false)
	a.activePhase = ""
	if !connecting && a.last != nil && e == "" && (a.last.Phase == "quality_check" || a.last.Phase == "diagnose" || a.last.Phase == "generate_reviewed" || a.last.Phase == "auto_onepass" || a.last.Phase == "auto") {
		if _, reportErr := a.ws.SaveHTMLReport(a.snapshot, *a.last); reportErr != nil && e == "" {
			e = "无法写入最新报告：" + reportErr.Error()
		}
	}
	// An automatic startup attempt must not block the user with a modal error.
	// Keep the exact diagnostic visible; manual retry may display a dialog.
	a.mu.Lock()
	wasAutomatic := connecting && a.autoConnecting
	if connecting {
		a.autoConnecting = false
	}
	a.mu.Unlock()
	if e != "" {
		if connecting {
			a.connected = false
			a.shared.connectionError = e
			set(a.ctl("auto"), "连接 Workbench")
		}
		a.progress = "连接失败，可人工重试"
		a.progressPercent = -1
		a.message("连接未完成（详情见任务日志）：" + shorten(e, 160))
		if !wasAutomatic {
			alert(e)
		}
		return
	}
	if connecting {
		set(a.ctl("port"), strconv.Itoa(a.port))
		_ = writeJSON(a.ws.path("last_connection.json"), map[string]any{"port": a.port})
		a.connected = true
		set(a.ctl("auto"), "查看细化方案")
		a.scannedSystem = ""
		a.preparedSystem = ""
		a.snapshot = nil
		a.proposals = nil
		a.qualityResult = nil
		a.qualitySummary = nil
		a.qualityStale = true
		a.qualityFingerprint = ""
		a.qualityMeshSignature = ""
		a.qualityProject = ""
		a.qualityQuickGeometrySignature = ""
		a.engineeringSummary = nil
		a.engineeringSyncedAt = time.Time{}
		a.shared = sharedDataRuntime{}
		a.namedRows = nil
		a.namedVisible = nil
		a.selectedNamed = nil
		a.pausedNamedSizes = nil
		a.namedGeometry = ""
		send(a.ctl("systems"), CB_RESETCONTENT, 0, 0)
		for _, s := range a.systems {
			add(a.ctl("systems"), s)
		}
		if len(a.systems) == 1 {
			send(a.ctl("systems"), CB_SETCURSEL, 0, 0)
		}
		a.shared.lastSelectedSystem = a.selectedMeshSystem()
		a.progress = "已连接 Workbench，可同步工程并检测质量"
		a.message(fmt.Sprintf("已找到 %d 个网格系统，打开 Meshing 后勾选确认并扫描", len(a.systems)))
		a.switchPage(0)
		return
	}
	if a.last != nil && (a.last.Phase == "quality_check" || a.last.Phase == "diagnose" || a.last.Phase == "generate_reviewed" || a.last.Phase == "auto_onepass" || a.last.Phase == "auto") {
		_ = a.loadQualityHistory()
	}
	if a.taskCenterOpen {
		a.taskCenterStatus = a.ws.taskStatus()
		a.taskCenterStatusUpdatedAt = time.Now()
		a.refreshTaskCenterText()
	}
	if a.last != nil {
		switch a.last.Phase {
		case "inspect":
			// Preserve locally entered group sizes across a read-only refresh,
			// but never carry them to a different CAD geometry or project.
			oldSizes := a.selectedNamed
			oldPaused := a.pausedNamedSizes
			oldGeometry := a.namedGeometry
			previousNamedRows := append([]Named(nil), a.namedRows...)
			a.namedRows = nil
			a.selectedNamed = map[string]float64{}
			a.pausedNamedSizes = map[string]float64{}
			if a.snapshot != nil {
				for _, n := range a.snapshot.Named {
					name := strings.TrimSpace(n.Name)
					if name == "" || len(n.IDs) == 0 {
						continue
					}
					a.namedRows = append(a.namedRows, n)
				}
				sort.Slice(a.namedRows, func(i, j int) bool {
					ai := strings.HasPrefix(a.namedRows[i].Name, "AM_")
					aj := strings.HasPrefix(a.namedRows[j].Name, "AM_")
					if ai != aj {
						return ai
					}
					return strings.ToLower(a.namedRows[i].Name) < strings.ToLower(a.namedRows[j].Name)
				})
			}
			if a.snapshot != nil {
				sameGeometry := oldGeometry != "" && oldGeometry == a.namedGeometryKey()
				a.namedGeometry = a.namedGeometryKey()
				if a.qualityFingerprint != "" && a.qualityFingerprint != a.snapshot.Fingerprint {
					a.qualityStale = true
				}
				a.advice = recommendNamedSizes(a.snapshot, displayedGlobalMM(a.snapshot, a.scannedBaseMM))
				a.sizeBuckets = groupAdvice(a.advice, nil)
				a.bucketCollapsed = map[int]bool{}
				if sameGeometry && a.planCustomized {
					a.selectedNamed, _ = rebindNamedSettingsByScope(oldSizes, previousNamedRows, a.namedRows)
					a.pausedNamedSizes, _ = rebindNamedSettingsByScope(oldPaused, previousNamedRows, a.namedRows)
					// Use the current in-memory selection. It is also stored on disk
					// so a process restart does not re-enable a disabled 8 mm group.
				} else {
					active, paused, _, found, loadErr := loadNamedPreferences(a.ws.path("group_preferences_v1.json"), a.snapshot, a.namedRows)
					if loadErr != nil {
						a.message("分组状态读取失败（没有覆盖原记录）：" + shorten(loadErr.Error(), 100))
					}
					if found && loadErr == nil {
						a.selectedNamed, a.pausedNamedSizes = active, paused
						a.planCustomized = true
					} else {
						a.selectedNamed = advisedActivation(a.advice, a.snapshot)
						a.pausedNamedSizes = map[string]float64{}
						a.planCustomized = false
					}
				}
				// Reflect actual manual and paused sizes in labels/grouping instead of
				// continuing to show an obsolete suggested 8 mm header.
				customSizes := map[string]float64{}
				for n, size := range a.pausedNamedSizes {
					customSizes[n] = size
				}
				for n, size := range a.selectedNamed {
					customSizes[n] = size
				}
				// Preserve native sizes of existing manually named inlet/outlet
				// controls when they are first enabled. Never overwrite an
				// explicit in-memory choice or saved active/paused preference.
				for _, n := range a.namedRows {
					if !strings.EqualFold(n.Name, "inlet") && !strings.EqualFold(n.Name, "outlet") {
						continue
					}
					if _, active := a.selectedNamed[n.Name]; active {
						continue
					}
					if _, paused := a.pausedNamedSizes[n.Name]; paused {
						continue
					}
					if mm, ok := existingNamedSizingSize(a.snapshot, n); ok {
						a.pausedNamedSizes[n.Name] = mm
						customSizes[n.Name] = mm
					}
				}
				a.sizeBuckets = groupAdvice(a.advice, customSizes)
			}
			a.redrawNamedRows()
			a.proposals = nil
			a.groupKeys = nil
			a.groupOverrides = map[string]float64{}
			a.groupDisabled = map[string]bool{}
			a.groupEnabled = map[string]bool{}
			// Existing Named Selection list is populated above; do not replace it with calculated geometry proposals.
			if entries, ok := a.report()["proposed_sizing"].([]any); ok {
				for _, entry := range entries {
					if group, ok := entry.(map[string]any); ok {
						key := printable(group["group_key"])
						if key == "—" || key == "" {
							continue
						}
						a.proposals = append(a.proposals, group)
						a.groupKeys = append(a.groupKeys, key)
						label := fmt.Sprintf("%02d  %s", len(a.groupKeys), groupLabel(group))
						_ = label
					}
				}
			}
			if len(a.namedRows) > 0 {
				a.syncGroupEditor()
			}
			for _, def := range []struct{ key, category string }{
				{"gap", "area_small"}, {"hole", "area_repeated"}, {"port", "named_existing"},
			} {
				if size, _, _, ok := a.proposed(def.category); ok {
					set(a.ctl("fine"+def.key+"mm"), strconv.FormatFloat(size, 'f', -1, 64))
				} else {
					set(a.ctl("fine"+def.key+"mm"), "")
				}
				send(a.ctl("fine"+def.key), BM_SETCHECK, 1, 0)
			}
			a.progress = "完整几何扫描已完成 · 请审核局部细化"
			a.message(fmt.Sprintf("已读取 %d 个现有命名组。仅对明确启用且经审核的命名组发布 Sizing（包含 inlet/outlet）。", len(a.namedRows)))
			a.switchPage(2)
			return
		case "publish_names":
			a.progress = "仅命名已完成 · 请在 Meshing 树中检查高亮范围"
			a.message("未创建 Sizing 或生成网格；要添加细化，请先刷新命名。")
			a.switchPage(2)
			return
		case "publish_existing_sizing":
			mapped := a.syncPublishedAliases()
			if removed, ok := a.report()["retired_managed"].([]any); ok && len(removed) > 0 {
				a.message(fmt.Sprintf("已验证停用 %d 个未勾选的 AutoMesh Sizing，控件仍保留在 Meshing 树中。", len(removed)))
			}
			a.progress = "按现有命名发布 Sizing 完成 · 请在 Meshing 树中检查"
			if mapped > 0 {
				a.message(fmt.Sprintf("已按完全相同的面范围安全识别 %d 个改名组，界面名称已同步；未重选面或重划网格。", mapped))
			} else {
				a.message("已复用现有 Named Selection，未新建命名或生成网格；可在 Meshing 直接编辑 Sizing。")
			}
			a.switchPage(2)
			return
		case "prepare":
			a.progress = "局部 Sizing 已发布 · 请在 Meshing 审核"
			if preserved, ok := a.report()["mesh_preserved"].(bool); ok && !preserved {
				a.message("ANSYS 在添加 Sizing 后改变了网格状态；请检查原网格是否失效。AutoMesh 没有清除或重划网格。")
			} else {
				a.message("未生成网格。请在 Meshing 树中检查 AM_* 与 AutoMesh_*，必要时修改尺寸和面作用域。")
			}
			a.switchPage(2)
			return
		case "rename_auxiliary":
			a.progress = "旧版辅助命名中文化完成，需重新扫描"
			a.message(fmt.Sprintf("中文化 %s 组；跳过 %s 组，详情见报告。原有 Sizing 未改。", printable(a.report()["renamed_count"]), printable(a.report()["skipped_count"])))
			a.switchPage(2)
			return
		case "warning_refine_preview":
			if a.lastSummary["status"] == "no_safe_candidates" {
				a.progress = "诊断完成：没有安全细化候选 · 可查看具体拒绝原因"
			} else {
				a.progress = "已生成警告区局部细化方案 · 仅只读分析"
			}
			a.message(fmt.Sprintf("共有 %s 个警告单元，建议 %s 个局部球形控制，尚有 %s 个未覆盖；请在报告中检查，并决定是否发布。", printable(a.report()["warning_count"]), printable(a.report()["group_count"]), printable(a.report()["uncovered_count"])))
			a.switchPage(3)
			return
		case "warning_refine_publish":
			a.progress = "所选警告区 Sizing 已发布 · 不会自动重划"
			added := printable(a.report()["sizing_count"])
			previous := printable(a.report()["existing_control_count"])
			total := printable(a.report()["cumulative_control_count"])
			remaining := printable(a.report()["remaining_count"])
			message := fmt.Sprintf("本次新增 %s 个 Sizing；保留历史 %s 个；当前累计 %s 个。本轮尚未发布的候选 %s 个仍可继续审核。Meshing 网格没有重划。", added, previous, total, remaining)
			if warning := strings.TrimSpace(printable(a.report()["snapshot_refresh_warning"])); warning != "" && warning != "—" {
				message += "；" + warning
			}
			a.message(message)
			// Keep the same geometry and current marker; only reviewed Sizing
			// controls were added. The live control inventory is refreshed by ACT.
			a.switchPage(3)
			return
		case "mark_failed", "mark_warning", "mark_high_skew", "mark_low_element_quality":
			status := printable(a.report()["status"])
			message := printable(a.report()["message"])
			groups := a.report()["created_groups"]
			a.progress = "Worksheet 元素定位：" + status
			a.message(message + "；新增选择组：" + printable(groups))
			a.switchPage(3)
			return
		case "quality_check":
			a.progress = "已读取当前网格质量；未修改网格"
			a.message("质量报告已更新：仅显示 ANSYS 实际返回的指标，无法获取的会注明原因。")
			a.switchPage(3)
			return
		case "diagnose":
			probe := asMap(a.report()["element_probe"])
			count := 0
			if samples, ok := probe["samples"].([]any); ok {
				count = len(samples)
			}
			a.progress = "诊断完成 · 风险元素采样清单已生成"
			a.message(fmt.Sprintf("本次采样定位 %d 个候选风险元素；单元 ID/坐标请看报告，未命中不代表无坏单元。", count))
			a.switchPage(3)
			if file := a.ws.path("quality_report.html"); file != "" {
				// Browser opens at the risk result instead of an unrelated report section.
				_ = exec.Command("cmd", "/c", "start", "", file).Start()
			}
			return
		case "export":
			a.progress = "已导出 .msh · 必须在 Fluent 复核质量和边界"
			a.message("导出完成；请在 Fluent 检查正交质量、偏度及边界分区。")
			return
		case "generate_reviewed", "auto_onepass", "auto":
			gate := printable(a.report()["quality_gate"])
			if gate == "preliminary_pass" {
				a.progress = "网格生成完毕 · 初步检查通过"
				a.message("已生成网格；仍需核查实际 CFD 质量、边界与片间覆盖率")
			} else {
				a.progress = "网格生成完毕 · 需质量复核"
				a.message("网格已生成但尚不能认定 CFD 合格；请查看质量报告与 Fluent 验证要求")
			}
			return
		}
	}
	a.progress = "任务已返回结果"
	a.message("ANSYS 任务已返回；请查看最新报告")
}

// syncPublishedAliases changes only in-memory labels after the ACT worker has
// confirmed a unique exact-scope rename; it cannot alter Meshing geometry.
func (a *state) syncPublishedAliases() int {
	raw, ok := a.report()["existing_named_sizing"].([]any)
	if !ok {
		return 0
	}
	mapped := map[string]string{}
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		old, _ := m["remapped_from"].(string)
		live, _ := m["named_selection"].(string)
		if old != "" && live != "" && old != live {
			mapped[old] = live
		}
	}
	if len(mapped) == 0 {
		return 0
	}
	for old, live := range mapped {
		if v, ok := a.selectedNamed[old]; ok {
			delete(a.selectedNamed, old)
			a.selectedNamed[live] = v
		}
		if v, ok := a.pausedNamedSizes[old]; ok {
			delete(a.pausedNamedSizes, old)
			a.pausedNamedSizes[live] = v
		}
	}
	for i := range a.namedRows {
		if live, ok := mapped[a.namedRows[i].Name]; ok {
			a.namedRows[i].Name = live
		}
	}
	if a.snapshot != nil {
		for i := range a.snapshot.Named {
			if live, ok := mapped[a.snapshot.Named[i].Name]; ok {
				a.snapshot.Named[i].Name = live
			}
		}
	}
	for i := range a.advice {
		if live, ok := mapped[a.advice[i].Name]; ok {
			a.advice[i].Name = live
		}
	}
	for i := range a.sizeBuckets {
		for j := range a.sizeBuckets[i].Members {
			if live, ok := mapped[a.sizeBuckets[i].Members[j]]; ok {
				a.sizeBuckets[i].Members[j] = live
			}
		}
	}
	a.persistNamedChoices()
	a.redrawNamedRows()
	return len(mapped)
}

func groupLabel(group map[string]any) string {
	kinds := map[string]string{"area_small": "小面积", "area_repeated": "重复面积",
		"area_review": "大面待审核", "named_existing": "人工命名"}
	families := map[string]string{"Plane": "平面", "Cylindrical": "圆柱面", "Cylinder": "圆柱面",
		"Cone": "圆锥面", "Sphere": "球面", "Torus": "环面", "AnalyticCurve": "解析曲面",
		"Freeform": "自由曲面", "UserSelection": "已有命名", "Other": "其他"}
	cat := printable(group["category"])
	family := printable(group["surface_family"])
	if v, ok := kinds[cat]; ok {
		cat = v
	}
	if v, ok := families[family]; ok {
		family = v
	}
	return fmt.Sprintf("%s · %s · %s 面 · %s–%s mm²", cat, family,
		printable(group["face_count"]), printable(group["area_mm2_min"]), printable(group["area_mm2_max"]))
}

func parseBucketMarker(s string) (int, bool) {
	if !strings.HasPrefix(s, "@@bucket:") {
		return 0, false
	}
	n, e := strconv.Atoi(strings.TrimPrefix(s, "@@bucket:"))
	return n, e == nil && n >= 0
}
func (a *state) selectedBucket() (int, bool) {
	idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
	if idx < 0 || idx >= len(a.namedVisible) {
		return 0, false
	}
	name := a.namedVisible[idx].Name
	if i, ok := parseBucketMarker(name); ok && i < len(a.sizeBuckets) {
		return i, true
	}
	for i, g := range a.sizeBuckets {
		for _, member := range g.Members {
			if member == name {
				return i, true
			}
		}
	}
	return 0, false
}

// Save explicitly changed UI draft selections; never call any Meshing API here.
// A checkbox change is only a draft. The earlier publication must not be
// treated as covering the latest requested selections and sizing sizes.
func (a *state) invalidatePreparedAfterEdit() {
	if a.preparedSystem != "" {
		a.preparedSystem = ""
		a.message("分组选择已更改；原 Meshing Sizing 仍存在。请复核旧控件并重新发布，再生成网格。")
	}
}

func (a *state) persistNamedChoices() {
	if a.snapshot == nil || len(a.namedRows) == 0 {
		return
	}
	pref, err := makeSavedNamedProfile(a.snapshot, a.namedRows, a.selectedNamed, a.pausedNamedSizes)
	if err == nil {
		err = saveNamedPreferences(a.ws.path("group_preferences_v1.json"), pref)
	}
	if err != nil {
		a.message("无法保存分组勾选状态：" + shorten(err.Error(), 120))
	}
}

func (a *state) syncGroupEditor() {
	idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
	set(a.ctl("finebulkmm"), "") // legacy editor retained hidden for compatibility only
	if idx < 0 || idx >= len(a.namedVisible) {
		set(a.ctl("finegroupmm"), "")
		set(a.ctl("finegroupsave"), "应用尺寸")
		set(a.ctl("finegroupactive"), "启用所选命名或整组")
		send(a.ctl("finegroupactive"), BM_SETCHECK, 0, 0)
		refresh()
		return
	}
	selected := a.namedVisible[idx]
	if i, isGroup := parseBucketMarker(selected.Name); isGroup && i < len(a.sizeBuckets) {
		g := a.sizeBuckets[i]
		n := bucketEnabledCount(a.selectedNamed, g.Members)
		set(a.ctl("finegroupmm"), strconv.FormatFloat(g.MM, 'f', -1, 64))
		set(a.ctl("finegroupsave"), "应用整组尺寸")
		if n == len(g.Members) && n > 0 {
			send(a.ctl("finegroupactive"), BM_SETCHECK, 1, 0)
		} else {
			send(a.ctl("finegroupactive"), BM_SETCHECK, 0, 0)
		}
		set(a.ctl("finegroupactive"), fmt.Sprintf("启用整组（%d/%d）", n, len(g.Members)))
		refresh()
		return
	}
	set(a.ctl("finegroupsave"), "应用单项尺寸")
	set(a.ctl("finegroupactive"), "启用此命名组")
	if mm, ok := a.selectedNamed[selected.Name]; ok {
		set(a.ctl("finegroupmm"), strconv.FormatFloat(mm, 'f', -1, 64))
		send(a.ctl("finegroupactive"), BM_SETCHECK, 1, 0)
	} else {
		suggested := a.pausedNamedSizes[selected.Name]
		if suggested <= 0 {
			for _, ad := range a.advice {
				if ad.Name == selected.Name {
					suggested = ad.MM
					break
				}
			}
		}
		if suggested > 0 {
			set(a.ctl("finegroupmm"), strconv.FormatFloat(suggested, 'f', -1, 64))
		} else {
			set(a.ctl("finegroupmm"), "")
		}
		send(a.ctl("finegroupactive"), BM_SETCHECK, 0, 0)
	}
	refresh()
}

// Show meaningful Chinese descriptions but keep original ANSYS names unchanged.
// Replacing an existing name could invalidate a manually bound Sizing.
func chineseNameHint(name string) string {
	switch {
	case strings.HasPrefix(name, "AM_") && strings.ContainsAny(name, "中文小面积重复圆柱面平曲待审孔壁窄缝"):
		return strings.TrimPrefix(name, "AM_")
	case strings.HasPrefix(name, "AM_Area_Cylinder_") || strings.HasPrefix(name, "AM_Repeat_Cylinder_") || strings.Contains(name, "_Cylinder_"):
		return "圆柱面候选"
	case strings.Contains(name, "_AnalyticCurve_"):
		return "解析曲面候选"
	case strings.Contains(name, "_Plane_"):
		return "平面候选"
	case strings.HasPrefix(name, "AM_"):
		return "自动命名组"
	case strings.EqualFold(name, "inlet"):
		return "进气边界（人工命名）"
	case strings.EqualFold(name, "outlet"):
		return "出气边界（人工命名）"
	default:
		return "已有人工命名"
	}
}
func (a *state) namedGeometryKey() string {
	if a.snapshot == nil {
		return ""
	}
	return a.snapshot.Project + "|" + a.snapshot.Fingerprint
}

func (a *state) redrawNamedRows() {
	lst := a.ctl("finegroups")
	previous := ""
	current := int(send(lst, LB_GETCURSEL, 0, 0))
	if current >= 0 && current < len(a.namedVisible) {
		previous = a.namedVisible[current].Name
	}
	byName := map[string]Named{}
	for _, n := range a.namedRows {
		byName[n.Name] = n
	}
	a.namedVisible = nil
	send(lst, LB_RESETCONTENT, 0, 0)
	selectedIndex := -1
	for i, g := range a.sizeBuckets {
		names := []string{}
		for _, n := range g.Members {
			if a.showAllNamed || strings.HasPrefix(n, "AM_") || strings.EqualFold(n, "inlet") || strings.EqualFold(n, "outlet") {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			continue
		}
		marker := fmt.Sprintf("@@bucket:%d", i)
		a.namedVisible = append(a.namedVisible, Named{Name: marker})
		prefix := "▼"
		if a.bucketCollapsed[i] {
			prefix = "▶"
		}
		addList(lst, fmt.Sprintf("%s %s", prefix, g.Label))
		if previous == marker {
			selectedIndex = len(a.namedVisible) - 1
		}
		if a.bucketCollapsed[i] {
			continue
		}
		for _, name := range names {
			ns, exists := byName[name]
			if !exists {
				continue
			}
			a.namedVisible = append(a.namedVisible, ns)
			prefix = "○"
			if _, enabled := a.selectedNamed[name]; enabled {
				prefix = "✓"
			}
			label := fmt.Sprintf("    %s  %s · %d 面", prefix, chineseNameHint(name), len(ns.IDs))
			if v, ok := a.selectedNamed[name]; ok {
				label += fmt.Sprintf(" · %.3g mm", v)
			}
			label += " · " + namedSizingCoverage(a.snapshot, ns)
			label += "  [" + shorten(name, 22) + "]"
			addList(lst, label)
			if name == previous {
				selectedIndex = len(a.namedVisible) - 1
			}
		}
	}
	if selectedIndex < 0 && len(a.namedVisible) > 0 {
		selectedIndex = 0
	}
	if selectedIndex >= 0 {
		send(lst, LB_SETCURSEL, uintptr(selectedIndex), 0)
	}
	a.syncGroupEditor()
}
func (a *state) resetRecommendations() {
	if a.snapshot == nil {
		alert("请先完整扫描当前 Meshing 几何")
		return
	}
	base := displayedGlobalMM(a.snapshot, a.scannedBaseMM)
	a.advice = recommendNamedSizes(a.snapshot, base)
	a.sizeBuckets = groupAdvice(a.advice, nil)
	a.bucketCollapsed = map[int]bool{}
	a.selectedNamed = advisedActivation(a.advice, a.snapshot)
	a.pausedNamedSizes = map[string]float64{}
	a.planCustomized = true // explicit reset is a user decision; persist the resulting selection
	a.invalidatePreparedAfterEdit()
	a.persistNamedChoices()
	a.redrawNamedRows()
	a.message(fmt.Sprintf("已生成 %d 个局部尺寸组；初步启用 %d 个命名组。发布前必须审核作用面和资源预算。", len(a.sizeBuckets), len(a.selectedNamed)))
}
func (a *state) toggleSelectedBucket() {
	i, ok := a.selectedBucket()
	if !ok {
		return
	}
	if a.bucketCollapsed == nil {
		a.bucketCollapsed = map[int]bool{}
	}
	a.bucketCollapsed[i] = !a.bucketCollapsed[i]
	a.redrawNamedRows()
}
func addList(h uintptr, s string) { send(h, LB_ADDSTRING, 0, ptr(s)) }

// Checkbox clicks immediately save enablement. Group bulk-size changes still
// require the separate explicit "整组设置" button to avoid overwriting child sizes.
func (a *state) toggleActiveSelection() {
	idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
	if idx < 0 || idx >= len(a.namedVisible) {
		return
	}
	if a.selectedNamed == nil {
		a.selectedNamed = map[string]float64{}
	}
	if a.pausedNamedSizes == nil {
		a.pausedNamedSizes = map[string]float64{}
	}
	enabled := send(a.ctl("finegroupactive"), BM_GETCHECK, 0, 0) == 1
	selected := a.namedVisible[idx].Name
	if i, header := parseBucketMarker(selected); header {
		if i >= len(a.sizeBuckets) {
			return
		}
		g := a.sizeBuckets[i]
		setBucketEnabled(a.selectedNamed, a.pausedNamedSizes, g.Members, a.advice, g.MM, enabled)
		a.message(fmt.Sprintf("%s %d 个命名：%s；选择已保存；发布后将核验并同步停用属于 AutoMesh 的旧 Sizing", g.Label, len(g.Members), map[bool]string{true: "已启用", false: "已停用"}[enabled]))
	} else {
		if !enabled {
			if mm, exists := a.selectedNamed[selected]; exists {
				a.pausedNamedSizes[selected] = mm
			}
			delete(a.selectedNamed, selected)
		} else {
			mm := a.pausedNamedSizes[selected]
			if mm <= 0 {
				for _, advice := range a.advice {
					if advice.Name == selected {
						mm = advice.MM
						break
					}
				}
			}
			if mm <= 0 {
				mm = 8
			}
			a.selectedNamed[selected] = mm
		}
		a.message(fmt.Sprintf("%s %s；选择已保存，下一次发布时同步当前选择并回读核验", shorten(selected, 36), map[bool]string{true: "已启用", false: "已停用"}[enabled]))
	}
	a.planCustomized = true
	a.invalidatePreparedAfterEdit()
	a.persistNamedChoices()
	a.redrawNamedRows()
}

func (a *state) saveBulkEditor() {
	idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
	if idx < 0 || idx >= len(a.namedVisible) {
		alert("请选择整组标题")
		return
	}
	i, ok := parseBucketMarker(a.namedVisible[idx].Name)
	if !ok || i >= len(a.sizeBuckets) {
		alert("请先选中左侧带 ▼ 的整组标题，再调整整组尺寸；子项只做单项修改")
		return
	}
	text := strings.TrimSpace(read(a.ctl("finegroupmm")))
	mm, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(mm, 0) || math.IsNaN(mm) || mm < 0.35 || mm > 500 {
		alert("整组尺寸必须是 0.35–500 mm 的有效数值")
		return
	}
	g := a.sizeBuckets[i]
	if a.selectedNamed == nil {
		a.selectedNamed = map[string]float64{}
	}
	if a.pausedNamedSizes == nil {
		a.pausedNamedSizes = map[string]float64{}
	}
	// Bulk SIZE editing must never toggle activation or alter group membership.
	setBucketSizePreservingSelection(a.selectedNamed, a.pausedNamedSizes, g.Members, mm)
	a.sizeBuckets[i].MM = mm
	label := "局部细化组"
	if mm <= 2 {
		label = "细网格组"
	} else if mm <= 5 {
		label = "中网格组"
	} else {
		label = "较粗网格／待审核组"
	}
	a.sizeBuckets[i].Label = fmt.Sprintf("%s %d · %.2g mm · %d 个命名", label, i+1, mm, len(g.Members))
	a.planCustomized = true
	a.invalidatePreparedAfterEdit()
	a.persistNamedChoices()
	a.redrawNamedRows()
	a.message(fmt.Sprintf("已对 %d 个命名组保存 %.3g mm；原有启停状态不变，尚未发布到 Meshing", len(g.Members), mm))
}
func (a *state) saveGroupEditor() {
	idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
	if idx < 0 || idx >= len(a.namedVisible) {
		alert("请先选择左侧命名或组")
		return
	}
	name := a.namedVisible[idx].Name
	if _, header := parseBucketMarker(name); header {
		alert("当前选中的是整组。请使用统一的「应用整组尺寸」按钮；展开后可对单项单独设置。")
		return
	}
	if a.selectedNamed == nil {
		a.selectedNamed = map[string]float64{}
	}
	if send(a.ctl("finegroupactive"), BM_GETCHECK, 0, 0) != 1 {
		alert("该命名组当前未启用。请先勾选「启用此命名组」，再应用尺寸；当前不会发布或写入 Meshing。")
		return
	}
	mm, err := strconv.ParseFloat(strings.TrimSpace(read(a.ctl("finegroupmm"))), 64)
	if err != nil || math.IsNaN(mm) || math.IsInf(mm, 0) || mm < 0.35 || mm > 500 {
		alert("请输入 0.35–500 mm 的有效尺寸")
		return
	}
	a.selectedNamed[name] = mm
	if a.pausedNamedSizes != nil {
		delete(a.pausedNamedSizes, name)
	}
	a.planCustomized = true
	a.invalidatePreparedAfterEdit()
	a.persistNamedChoices()
	a.redrawNamedRows()
	a.message(fmt.Sprintf("已单独设置 %s → %.3g mm；不影响同组其它命名", shorten(name, 28), mm))
}

func (a *state) act(id int) {
	if a.busy && (id == ID_FINE_GROUP_SAVE || id == ID_FINE_AUTO_RECOMMEND || id == ID_FINE_BULK_SET || id == ID_FINE_PREPARE || id == ID_FINE_GENERATE || id == ID_FINE_NAMES || id == ID_FINE_REFRESH || id == ID_SCAN || id == ID_CONNECT || id == ID_CHINESE_NAMES) {
		a.message("ANSYS 任务执行中，不能改动或重复提交当前方案")
		return
	}
	switch id {
	case ID_NAV_HOME:
		a.switchPage(0)
	case ID_NAV_CONNECT:
		a.switchPage(1)
	case ID_NAV_FINE:
		a.switchPage(2)
	case ID_NAV_REPORT:
		a.switchPage(3)
	case ID_NAV_ANALYSIS:
		a.switchPage(4)
	case ID_NAV_SETTINGS:
		a.switchPage(5)
	case ID_SYNC_STATUS:
		a.run("sync_status")
	case ID_TASK_CENTER:
		a.toggleTaskCenter()
	case ID_TASK_DRAWER_CLOSE:
		if a.taskCenterOpen {
			a.toggleTaskCenter()
		}
	case ID_ANALYSIS_DEEP_QUALITY:
		if !confirm("此操作只读采集当前 Meshing 网格的原生逐单元 Skewness，网格较大时可能耗时较长。\n若 ACT 不提供原生元素结果，将明确标记为未取得，不生成网格、不修改 Sizing。\n\n确认开始独立采集任务？") {
			return
		}
		a.run("quality_check_deep")
	case ID_ANALYSIS_REFRESH:
		a.refreshAnalysisHistory()
	case ID_ANALYSIS_EXPORT:
		a.openQualityHistoryCSV()
	case ID_ANALYSIS_REPORT:
		a.openQualityAnalysisReport()
	case ID_ACTION_HELP:
		a.showPageActionHelp()
	case ID_ANALYSIS_TAB_OVERVIEW:
		a.analysisTab = 0
		a.updateAnalysisRegionVisibility()
		refresh()
	case ID_ANALYSIS_TAB_COMPARE:
		a.analysisTab = 1
		a.updateAnalysisRegionVisibility()
		refresh()
	case ID_ANALYSIS_TAB_DISTRIBUTION:
		a.analysisTab = 2
		a.updateAnalysisRegionVisibility()
		refresh()
	case ID_ANALYSIS_TAB_REGION:
		a.analysisTab = 3
		a.updateAnalysisRegionVisibility()
		refresh()
	case ID_AUTO:
		if !a.connected {
			a.connect(false)
		} else if a.snapshot == nil || a.scannedSystem == "" {
			a.switchPage(2)
			a.message("请在本页执行完整几何扫描，再配置现有 Named Selection 的局部尺寸。")
		} else {
			a.switchPage(2)
		}
	case ID_FINE_GROUP_SAVE:
		idx := int(send(a.ctl("finegroups"), LB_GETCURSEL, 0, 0))
		if idx >= 0 && idx < len(a.namedVisible) {
			if _, isGroup := parseBucketMarker(a.namedVisible[idx].Name); isGroup {
				a.saveBulkEditor()
			} else {
				a.saveGroupEditor()
			}
		} else {
			alert("请先选择左侧命名或整组标题")
		}
	case ID_FINE_AUTO_RECOMMEND:
		if !confirm("重新计算推荐尺寸将重置本页未发布的人工尺寸设置。不会修改 Meshing。确认？") {
			return
		}
		a.resetRecommendations()
	case ID_FINE_BULK_SET:
		a.saveBulkEditor()
	case ID_FINE_FILTER:
		a.showAllNamed = !a.showAllNamed
		if a.showAllNamed {
			set(a.ctl("finefilter"), "只看 AutoMesh")
		} else {
			set(a.ctl("finefilter"), "显示全部命名")
		}
		a.redrawNamedRows()
	case ID_FINE_REFRESH:
		a.run("inspect")
	case ID_QUALITY_CHECK:
		a.run("quality_check")
	case ID_MARK_EXECUTE:
		switch int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0)) {
		case 0:
			a.run("mark_failed")
		case 1:
			a.run("mark_warning")
		case 2:
			a.run("mark_high_skew")
		case 3:
			a.run("mark_low_element_quality")
		default:
			alert("请选择一种目标单元标记规则")
		}
	case ID_CHINESE_NAMES:
		a.run("rename_auxiliary")
	case ID_FINE_NAMES:
		a.run("publish_names")
	case ID_FINE_PREPARE:
		a.run("publish_existing_sizing")
	case ID_FINE_GENERATE:
		a.run("generate_reviewed")
	case ID_FINE_GLOBAL_REVIEW:
		mm, _ := strconv.ParseFloat(strings.TrimSpace(read(a.ctl("size"))), 64)
		alert(globalReviewText(a.snapshot, mm, send(a.ctl("global"), BM_GETCHECK, 0, 0) == 1))
	case ID_TASK_STATUS:
		// A terminal ACT marker may be waiting for the 10-second safety window.
		// Reconcile only explicit terminal markers; never clear a live task lease.
		if err := a.ws.ReconcileFinishedLease(); err != nil {
			alert("任务状态恢复未完成：" + err.Error() + "\n没有重新投递，也没有操作网格。")
			return
		}
		alert(a.ws.taskStatus())
	case ID_TASK_STOP:
		a.requestStop()
	case ID_INIT_RETRY:
		a.beginInitialization()
	case ID_SCAN:
		a.run("inspect")
	case ID_CONNECT:
		a.connect(false)
	case ID_REPORT:
		a.switchPage(3)
	case ID_OPEN:
		if err := exec.Command("explorer.exe", a.ws.Dir).Start(); err != nil {
			alert(err.Error())
		}
	case ID_DIAGNOSE:
		a.run("diagnose")
	case ID_MARK:
		a.run("mark_failed")
	case ID_MARK_WARNING:
		a.run("mark_warning")
	case ID_MARK_HIGH_SKEW:
		a.run("mark_high_skew")
	case ID_WARNING_PLAN:
		a.run("warning_refine_preview")
	case ID_WARNING_PUBLISH:
		a.openCandidateReview()
	case ID_REVIEW_BACK:
		a.switchPage(3)
	case ID_REVIEW_SELECT_ALL:
		a.applyReviewBulkSelection(reviewGroupAll)
	case ID_REVIEW_SELECT_SEVERE:
		a.applyReviewBulkSelection(reviewGroupSevere)
	case ID_REVIEW_SELECT_WARNING:
		a.applyReviewBulkSelection(reviewGroupWarning)
	case ID_REVIEW_SELECT_OTHER:
		a.applyReviewBulkSelection(reviewGroupOther)
	case ID_REVIEW_WHY_DISABLED:
		a.showReviewPublishReason()
	case ID_REVIEW_CLEAR:
		a.clearReviewSelection()
	case ID_REVIEW_DETAILS:
		a.openCandidateReviewDetails()
	case ID_REVIEW_CONFIRM:
		a.confirmCandidateReview()
	case ID_WARNING_VIEW:
		a.openWarningPlanReport()
	case ID_EXPORT_MSH:
		a.run("export")
	case ID_HTML_REPORT:
		a.openDetailedReport()
	case ID_STORAGE_CHANGE:
		a.changeDataDirectory()
	case ID_CLEAR_LEASE:
		if a.busy {
			alert("当前窗口仍有任务进行中，不能解除占用")
			return
		}
		old, err := readJSON[Task](a.ws.taskLeasePath())
		if err != nil {
			alert("没有需要解除的任务占用，或记录无法读取：" + err.Error())
			return
		}
		if !confirm(fmt.Sprintf("危险操作：上次任务 %s（%s）未确认结束。\n\n请先到当前 ANSYS Meshing 检查已停止所有 AutoMesh/GenerateMesh 运行，不能仅凭界面没有反应就解除。\n\n确认已停止？", old.JobID, old.Phase)) {
			return
		}
		if !confirm("第二次确认：我已亲自确认旧 Meshing 任务已停止；理解若它仍在后台执行，解除占用并重新提交可能造成任务冲突。") {
			return
		}
		archive, e := a.ws.AbandonTaskLease("I_CONFIRMED_MESHING_STOPPED")
		if e != nil {
			alert("未解除任务锁：" + e.Error())
			return
		}
		alert("已存档旧任务至：" + archive + "。现在请重新执行完整几何扫描；若旧控制残留，按完全匹配恢复规则处理。")
	case ID_COPY, ID_TASK_DRAWER_LOG:
		a.openTaskLog()
	}
}
func (a *state) drawBtn(item *drawItem) {
	id := int(item.ID)
	r := item.RC
	nav := id >= ID_NAV_HOME && id <= ID_NAV_SETTINGS
	primary := id == ID_AUTO || id == ID_CONNECT || id == ID_FINE_GENERATE || id == ID_MARK_EXECUTE || id == ID_WARNING_PUBLISH || id == ID_REVIEW_CONFIRM || id == ID_QUALITY_CHECK
	danger := id == ID_TASK_STOP || id == ID_CLEAR_LEASE
	fillclr, fg := color(239, 243, 249), navy
	if primary {
		fillclr, fg = cobalt, white
	} else if nav {
		if id-ID_NAV_HOME == a.page {
			fillclr, fg = bluepale, cobalt
		} else {
			fillclr, fg = color(251, 252, 255), slate
		}
	} else if id >= ID_ANALYSIS_TAB_OVERVIEW && id <= ID_ANALYSIS_TAB_REGION {
		activeTab := map[int]int{ID_ANALYSIS_TAB_OVERVIEW: 0, ID_ANALYSIS_TAB_COMPARE: 1,
			ID_ANALYSIS_TAB_DISTRIBUTION: 2, ID_ANALYSIS_TAB_REGION: 3}[id]
		if a.analysisTab == activeTab {
			fillclr, fg = bluepale, cobalt
		} else {
			fillclr, fg = white, slate
		}
	} else if danger {
		fillclr, fg = stoppale, stopink
	}
	if item.State&ODS_SELECTED != 0 {
		if primary {
			fillclr = color(16, 72, 173)
		} else {
			fillclr = darkbg
		}
	}
	if item.State&ODS_DISABLED != 0 {
		fillclr, fg = color(244, 246, 249), color(145, 155, 170)
	}
	bx, by, bw, bh := int(r.L), int(r.T), int(r.R-r.L), int(r.B-r.T)
	if item.State&ODS_FOCUS != 0 {
		// Native owner-drawn buttons need an explicit keyboard focus indication.
		solid(item.HDC, bx, by, bw, bh, focusblue, 12)
		solid(item.HDC, bx+2, by+2, bw-4, bh-4, fillclr, 10)
	} else {
		solid(item.HDC, bx, by, bw, bh, fillclr, 12)
	}
	// Draw the actual native caption, so UI Automation and keyboard users
	// see exactly the same action text as sighted mouse users.
	s := read(item.HWND)
	sz, wt := 14, 500
	if primary {
		sz, wt = 15, 650
	}
	if nav {
		sz, wt = 14, 600
	}
	txt(item.HDC, s, r, sz, wt, fg, DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
}
func isActionID(id int) bool {
	return (id >= ID_NAV_HOME && id <= ID_REVIEW_DETAILS && id != ID_FINE_GROUP_SELECT) ||
		(id >= ID_REVIEW_SELECT_ALL && id <= ID_REVIEW_WHY_DISABLED) ||
		id == ID_ANALYSIS_REPORT || id == ID_ACTION_HELP ||
		(id >= ID_SYNC_STATUS && id <= ID_MARK_EXECUTE) || id == ID_STORAGE_CHANGE || id == ID_TASK_DRAWER_LOG
}
func wndproc(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_KEYDOWN:
		if w == 0x70 /*VK_F1*/ {
			ui.showPageActionHelp()
			return 0
		}
	case WM_GETMINMAXINFO:
		if l != 0 { // MINMAXINFO: ptReserved, ptMaxSize, ptMaxPosition, ptMinTrackSize
			*(*int32)(unsafe.Pointer(l + 24)) = 940
			*(*int32)(unsafe.Pointer(l + 28)) = 630
		}
		return 0
	case WM_ENTERSIZEMOVE:
		resizeActive = true
		return 0
	case WM_EXITSIZEMOVE:
		resizeActive = false
		ui.layout()
		refresh()
		return 0
	case WM_SIZE:
		// During interactive resize, defer child reposition until mouse release.
		// Expensive MoveWindow + owner-drawn buttons otherwise flood the UI queue.
		if !resizeActive {
			ui.layout()
		}
		return 0
	case WM_PAINT:
		paintStart := time.Now()
		ps := paintStruct{}
		dc := call(beginpaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		rc := rect{}
		call(getrect, hwnd, uintptr(unsafe.Pointer(&rc)))
		ww, hh := int(rc.R), int(rc.B)
		mem := backBuffer(dc, ww, hh)
		if mem != 0 {
			if resizeActive {
				// Simple cached background while dragging the window; full frame on release.
				solid(mem, 0, 0, ww, hh, panelbg, 0)
			} else {
				// Worker goroutines publish snapshot/report state under mu.
				// Render a consistent state and avoid concurrent map reads.
				ui.mu.Lock()
				ui.screen(mem, ww, hh)
				ui.mu.Unlock()
			}
			call(blit, dc, 0, 0, uintptr(ww), uintptr(hh), mem, 0, 0, 0x00CC0020)
		}
		call(endpaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		uiRepaints++
		if firstPaintMs < 0 {
			firstPaintMs = float64(time.Since(startedAt).Microseconds()) / 1000
		}
		if elapsed := time.Since(paintStart); elapsed > paintDurationMax {
			paintDurationMax = elapsed
		}
		return 0
	case WM_MOUSEWHEEL:
		// Wheel events over the blank panel scroll the current page; a native
		// ListBox retains its own scrolling when it receives the event.
		delta := int(int16(uint16((w >> 16) & 0xFFFF)))
		if delta != 0 {
			if w&0x4 != 0 { // SHIFT+wheel pans horizontally when necessary.
				if ui.scrollToX(ui.scrollX - (delta * 60 / 120)) {
					return 0
				}
			} else if ui.scrollTo(ui.scrollY - (delta * 60 / 120)) {
				return 0
			}
		}
	case WM_VSCROLL:
		rc := rect{}
		call(getrect, hwnd, uintptr(unsafe.Pointer(&rc)))
		code := int(w & 0xFFFF)
		next := ui.scrollY
		switch code {
		case SB_LINEUP:
			next -= 42
		case SB_LINEDOWN:
			next += 42
		case SB_PAGEUP:
			next -= int(rc.B) - 120
		case SB_PAGEDOWN:
			next += int(rc.B) - 120
		case SB_TOP:
			next = 0
		case SB_BOTTOM:
			next = 999999
		case SB_THUMBPOSITION, SB_THUMBTRACK:
			info := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: SIF_ALL}
			if call(getScrollInfo, hwnd, SB_VERT, uintptr(unsafe.Pointer(&info))) != 0 {
				next = int(info.TrackPos)
			}
		}
		if ui.scrollTo(next) {
			return 0
		}
	case WM_HSCROLL:
		rc := rect{}
		call(getrect, hwnd, uintptr(unsafe.Pointer(&rc)))
		code := int(w & 0xFFFF)
		next := ui.scrollX
		switch code {
		case SB_LINEUP:
			next -= 48
		case SB_LINEDOWN:
			next += 48
		case SB_PAGEUP:
			next -= int(rc.R) - 250
		case SB_PAGEDOWN:
			next += int(rc.R) - 250
		case SB_TOP:
			next = 0
		case SB_BOTTOM:
			next = 999999
		case SB_THUMBPOSITION, SB_THUMBTRACK:
			info := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: SIF_ALL}
			if call(getScrollInfo, hwnd, SB_HORZ, uintptr(unsafe.Pointer(&info))) != 0 {
				next = int(info.TrackPos)
			}
		}
		if ui.scrollToX(next) {
			return 0
		}
	case WM_DRAWITEM:
		if l != 0 {
			ui.drawBtn((*drawItem)(unsafe.Pointer(l)))
			return 1
		}
	case WM_NOTIFY:
		if l != 0 {
			meta := (*nmHeader)(unsafe.Pointer(l))
			// ListView emits many notifications besides checkbox changes.
			// Refreshing on paint/header notifications can cause a repaint loop.
			if meta.Window == ui.ctl("reviewlist") && meta.Code == LVN_ITEMCHANGED {
				ui.reviewSelectionSummary()
				return 0
			}
		}
	case WM_COMMAND:
		if l == ui.ctl("reviewrisk") && (w>>16) == BN_CLICKED {
			ui.reviewSelectionSummary()
			return 0
		}
		// Auto-checkbox has ID 0. Win32 BN_CLICKED must be routed by HWND,
		// otherwise its checked state gets overwritten when another row is selected.
		if l != 0 && l == ui.ctl("finegroupactive") && (w>>16) == 0 {
			if ui.busy {
				ui.syncGroupEditor()
				ui.message("当前任务执行中，禁止变更勾选状态")
				return 0
			}
			ui.toggleActiveSelection()
			return 0
		}
		if l == ui.ctl("ack") && (w>>16) == BN_CLICKED {
			ui.updateActionStates()
			return 0
		}
		if l == ui.ctl("systems") && (w>>16) == CBN_SELCHANGE {
			ui.onMeshSystemSelectionChanged()
			return 0
		}
		if (l == ui.ctl("analysisbase") || l == ui.ctl("analysistarget")) && (w>>16) == CBN_SELCHANGE {
			ui.refreshAnalysisRegionList()
			refresh()
			return 0
		}
		if l == ui.ctl("markrule") && (w>>16) == CBN_SELCHANGE {
			ui.onMarkerRuleChanged()
			return 0
		}
		if (l == ui.ctl("skewlow") || l == ui.ctl("skewhigh")) && (w>>16) == EN_CHANGE {
			// Inline range validation updates the native button immediately;
			// no need to submit a failing ACT job to discover a typo.
			ui.updateActionStates()
			return 0
		}
		id := int(w & 0xFFFF)
		if id == ID_FINE_GROUP_SELECT && (w>>16) == 2 { // LBN_DBLCLK
			ui.toggleSelectedBucket()
			return 0
		}
		if id == ID_FINE_GROUP_SELECT && (w>>16) == 1 { // LBN_SELCHANGE
			ui.syncGroupEditor()
			return 0
		}
		if isActionID(id) {
			ui.act(id)
			return 0
		}
	case WM_CTLCOLORBTN, WM_CTLCOLOREDIT, WM_CTLCOLORSTATIC:
		call(bkmode, w, 1)
		return call(gdi.NewProc("GetStockObject"), 0) // WHITE_BRUSH fallback returned by gdi? overwritten below
	case WM_APPSTATUS:
		ui.refreshTaskCenterText()
		refresh()
		return 0
	case WM_APPINIT:
		ui.mu.Lock()
		err := ui.initError
		ui.initRunning = false
		ui.initFinished = time.Now()
		ui.ready = err == ""
		ui.mu.Unlock()
		ui.updateActionStates()
		if err != "" {
			ui.message("内部脚本初始化失败：" + shorten(err, 90))
		} else {
			ui.message("内部脚本初始化成功，自动查找当前 Workbench 服务")
			// StartServer() is invoked only if no existing service can be verified.
			// Never start Workbench or click GUI coordinates; connect runs asynchronously.
			ui.connect(true)
		}
		return 0
	case WM_APPDONE:
		ui.finish(w == 1)
		if ui.taskCenterOpen {
			ui.taskCenterStatus = ui.ws.taskStatus()
			ui.taskCenterStatusUpdatedAt = time.Now()
			ui.refreshTaskCenterText()
		}
		return 0
	case WM_APPSTORAGE:
		ui.finishStorage()
		return 0
	case WM_DESTROY:
		disposeBuffer()
		releaseGDI()
		// Save a short local performance diagnostic after closing; no UI-thread disk I/O during painting.
		perf := map[string]any{"paint_count": uiRepaints, "paint_max_ms": float64(paintDurationMax.Microseconds()) / 1000,
			"back_buffer_allocations": backBitmapAllocations, "control_moves": layoutMoves, "version": "R8.1-Release-Candidate",
			"first_paint_ms": firstPaintMs, "nav_switch_max_ms": float64(navigationDurationMax.Microseconds()) / 1000,
			"layout_max_ms": float64(layoutDurationMax.Microseconds()) / 1000}
		_ = writeJSON(ui.ws.path("ui_performance.json"), perf)
		call(quit, 0)
		return 0
	}
	return call(defproc, hwnd, uintptr(msg), w, l)
}
func main() {
	runtime.LockOSThread()
	// GDI-scaled DPI-unaware context: Windows scales text and controls together
	// rather than leaving 150% text clipped by fixed pixel rectangles.
	// If unavailable on older builds Windows applies its default DPI behavior.
	if setDpiContext.Find() == nil {
		call(setDpiContext, ^uintptr(4))
	}
	oneInstance, _, mutexErr := createMutex.Call(0, 0, ptr("Local\\AutoMesh2024R2_SingleInstance"))
	if oneInstance == 0 {
		alert("无法创建 AutoMesh 单实例锁：" + mutexErr.Error())
		return
	}
	defer call(closeHandle, oneInstance)
	if mutexErr == syscall.Errno(183) {
		alert("AutoMesh 已在运行。请使用现有窗口，避免多个实例争用 Workbench 任务文件。")
		return
	}
	ws, wsErr := DefaultWorkspace()
	if wsErr != nil {
		alert(wsErr.Error())
		return
	}
	ui.ws = ws
	ui.instance = call(module, 0)
	name := ptr("AutoMesh2024R2Modern")
	appIcon := call(loadicon, ui.instance, 1) // RT_GROUP_ICON 1 embedded in PE resource
	if appIcon == 0 {
		appIcon = call(loadicon, 0, 32512)
	}
	c := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), Proc: syscall.NewCallback(wndproc), Instance: ui.instance, Icon: appIcon, Cursor: call(loadcursor, 0, 32512), Name: name, SmallIcon: appIcon}
	if call(register, uintptr(unsafe.Pointer(&c))) == 0 {
		alert("无法注册 Windows 主窗口")
		return
	}
	ui.window = call(create, 0, name, ptr("AutoMesh R8.1  |  六页面网格优化 · 发布候选版"), WS_OVERLAPPEDWINDOW|WS_VSCROLL|WS_HSCROLL, 70, 25, 1240, 730, 0, 0, ui.instance, 0)
	if ui.window == 0 {
		alert("无法创建 Windows 主窗口")
		return
	}
	// Build native controls off-screen and expose the window once, avoiding
	// two full dashboard paints and GDI churn during first launch.
	ui.build()
	ui.beginInitialization()
	call(show, ui.window, 1)
	call(update, ui.window)
	var m winMsg
	for {
		r := call(getmsg, uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) {
			break
		}
		// Native Tab/Shift+Tab navigation for the owner-drawn Win32 controls.
		// The main window is not a dialog resource, so ordinary TranslateMessage
		// alone does not provide reliable dialog-style tab traversal.
		if call(isDialogMsg, ui.window, uintptr(unsafe.Pointer(&m))) != 0 {
			if m.Msg == WM_KEYDOWN && m.W == VK_TAB {
				ui.revealKeyboardFocus()
			}
			continue
		}
		call(translate, uintptr(unsafe.Pointer(&m)))
		call(dispatch, uintptr(unsafe.Pointer(&m)))
	}
}
