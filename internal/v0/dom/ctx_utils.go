package dom

// uses ctx's CursorY if not provided
func GetCurrentPageWidth(ctx *LayoutContext, cursorY float64) float64 {
	if cursorY == 0 {
		cursorY = ctx.CursorY
	}
	for _, page := range ctx.Pages {
		if page.PageStartTotal <= cursorY && cursorY <= page.PageEndTotal {
			return page.Width - (page.PageMargin.Left + page.PageMargin.Right)
		}
	}
	return 0
}
