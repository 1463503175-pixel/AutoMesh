//go:build windows

package main

import "unsafe"

// Windows owns HRGN after a successful SetWindowRgn call. A separate child
// clip prevents native inputs from being painted on top of the pinned header.
// Cache clip bounds to avoid reallocating regions on every redraw/resize.
type childClip struct{ left, top, right, bottom int }

var childClips = map[uintptr]childClip{}

func clipScrolledControl(h uintptr, x, y, width, height, clientWidth, clientHeight int) {
	left := maxInt(0, 200-x)
	right := minIntScroll(width, clientWidth-x)
	top := maxInt(0, 62-y)
	bottom := minIntScroll(height, clientHeight-y)
	if bottom < top {
		bottom = top
	}
	if right < left {
		right = left
	}
	bounds := childClip{left: left, top: top, right: right, bottom: bottom}
	if previous, ok := childClips[h]; ok && previous == bounds {
		return
	}
	if left == 0 && right == width && top == 0 && bottom == height {
		call(setWindowRgn, h, 0, 0)
		childClips[h] = bounds
		return
	}
	rgn := call(createRectRgn, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
	if rgn == 0 {
		return
	}
	if call(setWindowRgn, h, rgn, 0) == 0 {
		call(deleteobj, rgn)
	} else {
		childClips[h] = bounds
	}
}
func (a *state) scrollTo(y int) bool {
	if a.window == 0 {
		return false
	}
	rc := rect{}
	call(getrect, a.window, uintptr(unsafe.Pointer(&rc)))
	clamped := clampScroll(y, a.page, int(rc.B))
	if clamped == a.scrollY {
		return false
	}
	a.scrollY = clamped
	a.layout()
	refresh()
	return true
}

func (a *state) scrollToX(x int) bool {
	if a.window == 0 {
		return false
	}
	rc := rect{}
	call(getrect, a.window, uintptr(unsafe.Pointer(&rc)))
	x = clampScrollX(x, int(rc.R))
	if x == a.scrollX {
		return false
	}
	a.scrollX = x
	a.layout()
	refresh()
	return true
}

// Tab navigation may focus a control that was previously outside the visible
// content pane. Reveal it without scrolling the pinned navigation/header.
func (a *state) revealKeyboardFocus() {
	if a.window == 0 {
		return
	}
	focused := call(getFocus)
	if focused == 0 {
		return
	}
	rc := rect{}
	call(getrect, a.window, uintptr(unsafe.Pointer(&rc)))
	for name, box := range plannedControls(int(rc.R)) {
		if a.ctl(name) != focused || !visibility[focused] || len(name) >= 3 && name[:3] == "nav" {
			continue
		}
		x, y := scrollToReveal(box, a.page, int(rc.R), int(rc.B), a.scrollX, a.scrollY)
		if x != a.scrollX || y != a.scrollY {
			a.scrollX = x
			a.scrollY = y
			a.layout()
			refresh()
		}
		return
	}
}
