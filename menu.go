package reline

import (
	"sort"
	"strings"
)

// MenuLines lays out completion candidates for a terminal screenWidth
// columns wide (MRI Reline::LineEditor::MenuInfo#lines): sorted, padded to
// the widest candidate plus two spaces, in as many columns as fit (at least
// one), filled column by column, each row's trailing spaces trimmed.
func MenuLines(list []string, screenWidth int) []string {
	if len(list) == 0 {
		return nil
	}
	sorted := append([]string(nil), list...)
	sort.Strings(sorted)
	sizes := make([]int, len(sorted))
	itemWidth := 0
	for i, item := range sorted {
		sizes[i] = CalculateWidth(item, false)
		itemWidth = max(itemWidth, sizes[i])
	}
	itemWidth += 2
	numCols := max(screenWidth/itemWidth, 1)
	numRows := (len(sorted) + numCols - 1) / numCols
	rows := make([]string, numRows)
	for i, item := range sorted {
		rows[i%numRows] += item + strings.Repeat(" ", itemWidth-sizes[i])
	}
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return rows
}

// TakeMenu returns the pending completion menu, as MenuInfo does, and clears
// it, as MRI's render does once it has drawn the menu: a host draws the
// candidates once, below the line, and the next key starts afresh.
func (le *LineEditor) TakeMenu() ([]string, bool) {
	list, ok := le.menuInfo, le.hasMenu
	le.menuInfo, le.hasMenu = nil, false
	return list, ok
}
