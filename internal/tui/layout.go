package tui

type layoutMode int

const (
	layoutCompact layoutMode = iota
	layoutStandard
	layoutWide
)

type layout struct {
	mode           layoutMode
	width          int
	listWidth      int
	detailWidth    int
	viewportHeight int
}

func calculateLayout(width, height int) layout {
	result := layout{mode: layoutCompact, width: max(width, 1), listWidth: max(width, 1)}
	if width >= 160 {
		result.mode = layoutWide
		result.detailWidth = 50
		result.listWidth = max(width-result.detailWidth-2, 1)
	} else if width >= 110 {
		result.mode = layoutWide
		result.detailWidth = 46
		result.listWidth = max(width-result.detailWidth-2, 1)
	} else if width >= 72 {
		result.mode = layoutStandard
	}
	result.viewportHeight = max(height-11, 1)
	if result.mode == layoutStandard {
		result.viewportHeight = max(height-12, 1)
	} else if result.mode == layoutWide {
		// The wide header is logo-bound: it keeps its full height with or
		// without the update notice, so the list always reserves that row.
		result.viewportHeight = max(height-13, 1)
	}
	return result
}
