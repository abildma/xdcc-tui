package tui

import (
	"fmt"
	"strings"
)

// applyFilter filters the search results based on the filter input
func (m *Model) applyFilter() {
	filterText := strings.ToLower(m.filterInput.Value())
	if filterText == "" {
		// If filter is empty, show all results
		m.filteredResults = m.searchResults
		m.status = fmt.Sprintf("Showing all %d results", len(m.searchResults))
	} else {
		// Filter results based on the filter text
		m.filteredResults = []FileItem{}
		for _, item := range m.searchResults {
			if strings.Contains(strings.ToLower(item.name), filterText) {
				m.filteredResults = append(m.filteredResults, item)
			}
		}
		m.status = fmt.Sprintf("Found %d results containing '%s'", len(m.filteredResults), filterText)
	}
	
	// Reset page to 0
	m.page = 0
}
