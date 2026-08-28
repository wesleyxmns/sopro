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
	if width >= 110 {
		result.mode = layoutWide
		result.detailWidth = 46
		result.listWidth = max(width-result.detailWidth-2, 1)
	} else if width >= 72 {
		result.mode = layoutStandard
	}
	result.viewportHeight = max(height-9, 1)
	if result.mode != layoutCompact {
		result.viewportHeight = max(height-10, 1)
	}
	return result
}
