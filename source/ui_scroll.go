package main

func pageContentBottom(page int) int {
	switch page {
	case 0:
		return 691
	case 1:
		return 681
	case 2:
		return 703
	case 3:
		return 711
	case 4:
		return 1180
	case 5:
		return 671
	case 6:
		return 720
	default:
		return 711
	}
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minIntScroll(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func scrollLimit(page, height int) int {
	return maxInt(0, pageContentBottom(page)+12-height)
}
func clampScroll(y, page, height int) int {
	if y < 0 {
		return 0
	}
	return minIntScroll(y, scrollLimit(page, height))
}

// Preserve the existing 860 logical-pixel content grid: when the actual
// client becomes narrower, reveal the content with a horizontal scrollbar.
func scrollLimitX(width int) int { return maxInt(0, contentX+uiWidth(width)+20-width) }
func clampScrollX(x, width int) int {
	if x < 0 {
		return 0
	}
	return minIntScroll(x, scrollLimitX(width))
}

// scrollToReveal computes the minimal offset needed to reveal a keyboard-
// focused native control within the pinned header/sidebar viewport. Both
// offsets are clamped independently to the page's safe virtual scroll limits.
func scrollToReveal(box uiBox, page, width, height, oldX, oldY int) (int, int) {
	x, y := oldX, oldY
	if box.Y-y < 70 {
		y = box.Y - 70
	}
	if box.Y+box.H-y > height-18 {
		y = box.Y + box.H - (height - 18)
	}
	if box.X-x < 207 {
		x = box.X - 207
	}
	if box.X+box.W-x > width-18 {
		x = box.X + box.W - (width - 18)
	}
	return clampScrollX(x, width), clampScroll(y, page, height)
}
