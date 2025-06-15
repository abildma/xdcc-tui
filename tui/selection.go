//go:build ignore
// +build ignore

package tui

import (
	"fmt"
	"xdcc-tui/xdcc"
)

// SelectionHelper provides additional functionality for file selection
// This is used to ensure files selected with Space are properly downloaded
func (m *Model) AddSelectedFilesToQueue() {
	// First, completely clear the download queue
	m.downloadQueue = []*xdcc.IRCFile{}
	
	// Count explicitly selected files
	selectedCount := 0
	for _, item := range m.filteredResults {
		if item.selected {
			selectedCount++
		}
	}
	
	// Debug information
	fmt.Printf("Selection check: found %d explicitly selected files\n", selectedCount)
	
	// Only use explicitly selected files if any exist
	if selectedCount > 0 {
		// Add all selected files to the queue
		for _, item := range m.filteredResults {
			if item.selected && item.url != nil {
				m.downloadQueue = append(m.downloadQueue, item.url)
				fmt.Printf("Adding to queue: %s\n", item.name)
			}
		}
		
		m.status = fmt.Sprintf("Selected %d files for download", len(m.downloadQueue))
	} else {
		// Fallback: use cursor position
		startIdx := m.page * m.itemsPerPage
		cursorIdx := startIdx + m.cursor
		
		// Ensure we don't go out of bounds
		if cursorIdx < len(m.filteredResults) {
			cursorItem := m.filteredResults[cursorIdx]
			
			if cursorItem.url != nil {
				m.downloadQueue = append(m.downloadQueue, cursorItem.url)
				m.status = fmt.Sprintf("Downloading file at cursor: %s", cursorItem.name)
			}
		}
	}
}
