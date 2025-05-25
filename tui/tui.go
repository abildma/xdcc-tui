package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xdcc-tui/search"
	"xdcc-tui/xdcc"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mode represents the current mode of the TUI
type Mode int

const (
	ModeSearch Mode = iota
	ModeResults
	ModeDownloading
	ModeFilter
)

// FileItem represents a file in search results
type FileItem struct {
	name     string
	size     int64
	url      *xdcc.IRCFile
	selected bool
}

// List item interface implementations
func (i FileItem) Title() string {
	sizeStr := ""
	if i.size > 0 {
		sizeStr = fmt.Sprintf("(%d KB)", i.size/1024)
	}
	return fmt.Sprintf("%s %s", i.name, sizeStr)
}

func (i FileItem) Description() string {
	return i.url.String()
}

func (i FileItem) FilterValue() string {
	return i.name
}

// Helper to find a file item by URL
func findFileItemByURL(items []FileItem, url *xdcc.IRCFile) *FileItem {
	for i, item := range items {
		if item.url.String() == url.String() {
			return &items[i]
		}
	}
	return nil
}

// Message types for the TUI
type searchResultMsg struct {
	results []FileItem
}

type errorMsg struct {
	err error
	url *xdcc.IRCFile
}

type downloadProgressMsg struct {
	bytesDownloaded int64
	totalBytes      int64
	url             *xdcc.IRCFile
	speed           float64
}

type downloadFinishedMsg struct {
	url *xdcc.IRCFile
}

// Model represents the TUI state
type Model struct {
	mode            Mode
	searchInput     textinput.Model
	filterInput     textinput.Model
	spinner         spinner.Model
	progress        progress.Model
	searchResults   []FileItem
	filteredResults []FileItem
	cursor          int
	page            int
	itemsPerPage    int
	searchEngine    *search.ProviderAggregator
	error           string
	status          string
	// Change to map[string]bool to use URL strings as keys for more reliable tracking
	selectedFiles   map[string]bool
	downloadQueue   []*xdcc.IRCFile
	queueCursor     int
	downloadPaused  bool
	currentFile     string
	downloadedSize  int64
	totalSize       int64
	lastDownloadURL *xdcc.IRCFile
}

// NewModel creates a new model
func NewModel() Model {
	// Initialize text input for search
	searchInput := textinput.New()
	searchInput.Placeholder = "Enter search terms..."
	searchInput.Focus()

	// Initialize filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Enter filter terms..."

	// Initialize spinner for loading states
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Initialize progress bar
	p := progress.New(progress.WithDefaultGradient())

	// Create search engine
	searchEngine := search.NewProviderAggregator(
		&search.XdccEuProvider{},
		&search.XdccServProvider{},
	)

	// Initialize model
	m := Model{
		mode:           ModeSearch,
		searchInput:    searchInput,
		filterInput:    filterInput,
		spinner:        s,
		progress:       p,
		searchEngine:   searchEngine,
		cursor:         0,
		page:           0,
		itemsPerPage:   15,
		selectedFiles:  make(map[string]bool),
		downloadQueue:  make([]*xdcc.IRCFile, 0),
		downloadPaused: false,
	}

	// Create downloads directory if it doesn't exist
	os.MkdirAll("downloads", 0755)

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

// Update handles user input and events
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Special handling for search and filter modes
		// Pass input to text fields first when in these modes
		if m.mode == ModeSearch {
			// Handle special keys first
			switch msg.String() {
			case "ctrl+c", "esc":
				// If we have results, exit to results mode, otherwise quit
				if len(m.searchResults) > 0 {
					m.mode = ModeResults
					m.searchInput.Blur()
					m.status = "Switched to results mode"
					return m, nil
				} else {
					// No results, so just quit
					return m, tea.Quit
				}
			case "tab":
				// Switch to results mode if we have results
				if len(m.searchResults) > 0 {
					m.mode = ModeResults
					m.searchInput.Blur()
					return m, nil
				}
			case "enter":
				// Submit search
				m.status = "Searching for " + m.searchInput.Value() + "..."
				return m, func() tea.Msg {
					// Split search query into keywords
					keywords := strings.Fields(m.searchInput.Value())
					results, err := m.searchEngine.Search(keywords)
					if err != nil {
						return errorMsg{err: err}
					}

					// Convert to FileItems
					fileItems := []FileItem{}
					for _, r := range results {
						fileItems = append(fileItems, FileItem{
							name:     r.Name,
							size:     r.Size,
							url:      &r.URL,
							selected: false,
						})
					}

					// ALWAYS switch to results mode after search
					m.mode = ModeResults
					m.searchInput.Blur()

					// Add status message
					if len(fileItems) == 0 {
						m.status = "No results found for: " + m.searchInput.Value()
					} else {
						m.status = fmt.Sprintf("Found %d results", len(fileItems))
					}

					// Clear the selection map when loading new search results
					m.selectedFiles = make(map[string]bool)
					m.searchResults = fileItems
					m.filteredResults = fileItems
					m.cursor = 0
					m.page = 0

					return searchResultMsg{results: fileItems}
				}
			default:
				// Pass all other keys to the search input
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		} else if m.mode == ModeFilter {
			// Handle special keys first
			switch msg.String() {
			case "ctrl+c", "esc":
				// Exit filter mode to results mode
				m.mode = ModeResults
				m.filterInput.Blur()
				m.status = "Filter mode exited"
				return m, nil
			case "enter":
				// Apply filter
				m.mode = ModeResults
				m.filterInput.Blur()
				m.status = "Filter applied"
				return m, nil
			default:
				// Pass all other keys to the filter input
				m.filterInput, cmd = m.filterInput.Update(msg)

				// Apply filter as you type
				filterText := m.filterInput.Value()
				if filterText == "" {
					m.filteredResults = m.searchResults
					m.status = fmt.Sprintf("Showing all %d results", len(m.searchResults))
				} else {
					m.filteredResults = []FileItem{}
					for _, item := range m.searchResults {
						if strings.Contains(strings.ToLower(item.name), strings.ToLower(filterText)) {
							m.filteredResults = append(m.filteredResults, item)
						}
					}
					m.status = fmt.Sprintf("Found %d matching results", len(m.filteredResults))
				}

				// Reset page and cursor
				m.page = 0
				m.cursor = 0

				return m, cmd
			}
		}

		// For other modes, handle keys normally
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			if m.mode == ModeResults {
				// Start download of ONLY explicitly selected files
				var selectedItems []FileItem

				// Look for files marked as selected in our central tracking map
				fmt.Printf("DEBUG: Checking for selected files among %d filtered results\n", len(m.filteredResults))

				// First, directly check which files are in our selection map
				fmt.Printf("DEBUG: Selected files in tracking map:\n")
				for urlStr, selected := range m.selectedFiles {
					if selected {
						fmt.Printf("DEBUG: URL %s is selected in the map\n", urlStr)
					}
				}

				// Now check the items and ensure they match our selection map
				for i, item := range m.filteredResults {
					urlStr := item.url.String()

					// Extra debug output to help diagnose selection issues
					if i < 10 { // Limit output to first 10 items to avoid flooding
						fmt.Printf("DEBUG: File #%d: '%s' selection status: item=%v, map=%v\n",
							i, item.name, item.selected, m.selectedFiles[urlStr])
					}

					// Use the map for source of truth, but also check the item
					if m.selectedFiles[urlStr] || item.selected {
						// Make sure both are in sync
						if !item.selected {
							fmt.Printf("DEBUG: Fixing selection inconsistency for %s\n", item.name)
							item.selected = true
						}
						if !m.selectedFiles[urlStr] {
							m.selectedFiles[urlStr] = true
						}

						selectedItems = append(selectedItems, item)
						fmt.Printf("Found selected file: %s\n", item.name)
					}
				}

				if len(selectedItems) > 0 {
					// Add selected files to download queue
					for _, item := range selectedItems {
						m.downloadQueue = append(m.downloadQueue, item.url)
					}

					// Switch to download mode
					m.mode = ModeDownloading
					m.status = fmt.Sprintf("Added %d files to download queue", len(selectedItems))

					// Start download if not already downloading
					if len(m.downloadQueue) > 0 && m.downloadedSize == 0 {
						// Ensure downloads directory exists
						os.MkdirAll("downloads", 0755)

						// Start first download
						outputPath := filepath.Join("downloads", filepath.Base(m.downloadQueue[0].String()))
						cmds = append(cmds, downloadFile(m.downloadQueue[0], outputPath))
					}
				} else {
					m.status = "No files selected for download"
				}
				return m, tea.Batch(cmds...)
			}

		case " ", "space":
			// Toggle selection of current item
			if m.mode == ModeResults {
				// Get current page items and cursor position
				startIdx := m.page * m.itemsPerPage
				endIdx := startIdx + m.itemsPerPage
				if endIdx > len(m.filteredResults) {
					endIdx = len(m.filteredResults)
				}

				// Calculate actual index in the full results array
				actualIdx := startIdx + m.cursor
				fmt.Printf("DEBUG: Space pressed at cursor: %d, actual index: %d\n", m.cursor, actualIdx)

				// Make sure we're within bounds of the results
				if actualIdx >= 0 && actualIdx < len(m.filteredResults) {
					// Get the current item directly
					item := &m.filteredResults[actualIdx]
					urlStr := item.url.String()

					// Toggle the selection state in the map (our source of truth)
					currentlySelected := m.selectedFiles[urlStr]
					if currentlySelected {
						// Remove from selected files
						delete(m.selectedFiles, urlStr)
						// Also update the item's selected flag for consistency
						item.selected = false
						fmt.Printf("DEBUG: Removing %s from selection map\n", urlStr)
					} else {
						// Add to selected files
						m.selectedFiles[urlStr] = true
						// Also update the item's selected flag for consistency
						item.selected = true
						fmt.Printf("DEBUG: Adding %s to selection map\n", urlStr)
					}

					fmt.Printf("DEBUG: Toggled selection for '%s' to: %v\n", item.name, item.selected)

					// Update status message with selection state
					if item.selected {
						m.status = fmt.Sprintf("Selected: %s", item.name)
					} else {
						m.status = fmt.Sprintf("Unselected: %s", item.name)
					}
				} else {
					// Log error if we're out of bounds
					fmt.Printf("ERROR: Invalid index %d for selection, results length: %d\n", actualIdx, len(m.filteredResults))
					m.status = "Error: Could not select item, invalid position"
				}
			}

		case "j", "down":
			// Move cursor down
			if m.mode == ModeResults {
				startIdx := m.page * m.itemsPerPage
				endIdx := startIdx + m.itemsPerPage
				if endIdx > len(m.filteredResults) {
					endIdx = len(m.filteredResults)
				}

				m.cursor++
				if startIdx+m.cursor >= endIdx {
					m.cursor = endIdx - startIdx - 1
					if m.cursor < 0 {
						m.cursor = 0
					}
				}
			} else if m.mode == ModeDownloading {
				// Move cursor in download queue
				m.queueCursor++
				if m.queueCursor >= len(m.downloadQueue) {
					m.queueCursor = len(m.downloadQueue) - 1
					if m.queueCursor < 0 {
						m.queueCursor = 0
					}
				}
			}

		case "k", "up":
			// Move cursor up
			if m.mode == ModeResults {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = 0
				}
			} else if m.mode == ModeDownloading {
				// Move cursor in download queue
				m.queueCursor--
				if m.queueCursor < 0 {
					m.queueCursor = 0
				}
			}

		case "h", "left":
			// Previous page
			if m.mode == ModeResults && m.page > 0 {
				m.page--
				m.cursor = 0
				m.status = fmt.Sprintf("Page %d of %d", m.page+1, (len(m.filteredResults)-1)/m.itemsPerPage+1)
			}

		case "l", "right":
			// Next page
			if m.mode == ModeResults {
				maxPage := (len(m.filteredResults) - 1) / m.itemsPerPage
				if m.page < maxPage {
					m.page++
					m.cursor = 0
					m.status = fmt.Sprintf("Page %d of %d", m.page+1, maxPage+1)
				}
			}

		case "d":
			// Remove item from download queue
			if m.mode == ModeDownloading && len(m.downloadQueue) > 0 && m.queueCursor < len(m.downloadQueue) {
				// Save the item to show in status
				itemToRemove := m.downloadQueue[m.queueCursor]

				// Get readable filename for status message
				fileName := "unknown file"
				fileItem := findFileItemByURL(m.searchResults, itemToRemove)
				if fileItem != nil {
					fileName = fileItem.name
				}

				// Remove from queue
				m.downloadQueue = append(m.downloadQueue[:m.queueCursor], m.downloadQueue[m.queueCursor+1:]...)

				// Adjust cursor if needed
				if m.queueCursor >= len(m.downloadQueue) && len(m.downloadQueue) > 0 {
					m.queueCursor = len(m.downloadQueue) - 1
				}

				m.status = fmt.Sprintf("Removed %s from download queue", fileName)
			}

		case "s":
			// Search mode
			if m.mode != ModeSearch {
				m.mode = ModeSearch
				m.searchInput.Focus()
				m.filterInput.Blur()
				m.status = "Enter search terms"
			}

		case "f":
			// Filter mode - only enter filter mode if we're in results mode
			if m.mode == ModeResults {
				m.mode = ModeFilter
				m.filterInput.Focus()
				m.searchInput.Blur()
				m.status = "Enter filter terms"
			}

		case "tab":
			// Toggle between modes - works from any mode
			if m.mode == ModeSearch {
				// From search -> results (if we have results)
				if len(m.searchResults) > 0 {
					m.mode = ModeResults
					m.searchInput.Blur()
					m.status = "Switched to results mode"
				}
			} else if m.mode == ModeResults {
				// From results -> downloads
				m.mode = ModeDownloading
				m.status = "Switched to download queue view"
			} else if m.mode == ModeDownloading {
				// From downloads -> search
				m.mode = ModeSearch
				m.searchInput.Focus()
				m.status = "Enter search terms"
			}

		default:
			// Any other key press
			if m.mode == ModeSearch {
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			} else if m.mode == ModeFilter {
				m.filterInput, cmd = m.filterInput.Update(msg)

				// Apply filter as you type
				filterText := m.filterInput.Value()
				if filterText == "" {
					m.filteredResults = m.searchResults
					m.status = fmt.Sprintf("Showing all %d results", len(m.searchResults))
				} else {
					m.filteredResults = []FileItem{}
					for _, item := range m.searchResults {
						if strings.Contains(strings.ToLower(item.name), strings.ToLower(filterText)) {
							m.filteredResults = append(m.filteredResults, item)
						}
					}
					m.status = fmt.Sprintf("Found %d matching results", len(m.filteredResults))
				}

				// Reset page and cursor
				m.page = 0
				m.cursor = 0

				return m, cmd
			}
		}

	case searchResultMsg:
		// Handle search results
		m.searchResults = msg.results
		m.filteredResults = msg.results
		if len(msg.results) > 0 {
			m.status = fmt.Sprintf("Found %d results", len(msg.results))
		} else {
			m.status = "No results found"
		}
		m.page = 0
		m.cursor = 0
		return m, nil

	case errorMsg:
		// Handle errors
		m.error = msg.err.Error()
		if msg.url != nil {
			// Error during download, remove from queue and try next
			for i, url := range m.downloadQueue {
				if url.String() == msg.url.String() {
					m.downloadQueue = append(m.downloadQueue[:i], m.downloadQueue[i+1:]...)
					break
				}
			}

			// Start next download if available
			if len(m.downloadQueue) > 0 {
				outputPath := filepath.Join("downloads", filepath.Base(m.downloadQueue[0].String()))
				cmds = append(cmds, downloadFile(m.downloadQueue[0], outputPath))
				return m, tea.Batch(cmds...)
			}
		}
		m.status = "Error: " + m.error
		return m, nil

	case downloadProgressMsg:
		// Update download progress
		m.downloadedSize = msg.bytesDownloaded
		m.totalSize = msg.totalBytes
		m.lastDownloadURL = msg.url

		// Create status message
		fileItem := findFileItemByURL(m.searchResults, msg.url)
		if fileItem != nil {
			m.currentFile = fileItem.name
		} else {
			m.currentFile = msg.url.String()
		}

		// Update status
		if msg.totalBytes > 0 {
			percent := float64(msg.bytesDownloaded) / float64(msg.totalBytes) * 100
			m.status = fmt.Sprintf("Downloading %s: %.1f%% (%.1f KB/s)",
				m.currentFile,
				percent,
				float64(msg.speed)/1024)
		} else {
			m.status = fmt.Sprintf("Downloading %s: %d KB (%.1f KB/s)",
				m.currentFile,
				msg.bytesDownloaded/1024,
				float64(msg.speed)/1024)
		}

		return m, nil

	case downloadFinishedMsg:
		// Handle finished download
		// Remove from queue
		for i, url := range m.downloadQueue {
			if url.String() == msg.url.String() {
				m.downloadQueue = append(m.downloadQueue[:i], m.downloadQueue[i+1:]...)
				break
			}
		}

		// Reset download stats
		m.downloadedSize = 0
		m.totalSize = 0

		// Get filename for status
		fileName := msg.url.String()
		fileItem := findFileItemByURL(m.searchResults, msg.url)
		if fileItem != nil {
			fileName = fileItem.name
		}

		// Update status
		m.status = fmt.Sprintf("Download completed: %s", fileName)

		// Start next download if available
		if len(m.downloadQueue) > 0 {
			outputPath := filepath.Join("downloads", filepath.Base(m.downloadQueue[0].String()))
			cmds = append(cmds, downloadFile(m.downloadQueue[0], outputPath))
			return m, tea.Batch(cmds...)
		}

		return m, nil

	case spinner.TickMsg:
		// Update spinner
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Return updated model
	return m, nil
}

// View renders the UI
func (m Model) View() string {
	var s strings.Builder

	// Header
	s.WriteString("💾 XDCC-TUI - Interactive XDCC Downloader 💾\n\n")

	// Mode display
	switch m.mode {
	case ModeSearch:
		s.WriteString("📡 Search Mode 📡\n\n")
		s.WriteString(fmt.Sprintf("Enter search terms: %s\n", m.searchInput.View()))
		if m.status != "" {
			s.WriteString("\n" + m.status + "\n")
		}
		if m.error != "" {
			s.WriteString("\nError: " + m.error + "\n")
		}
		if len(m.searchResults) > 0 {
			s.WriteString(fmt.Sprintf("\nFound %d results\n", len(m.searchResults)))
		}
		s.WriteString("\ns: Search | tab: Switch Mode | esc: Quit\n")

	case ModeResults:
		s.WriteString("📄 Results Mode 📄\n\n")

		// Current page info
		totalPages := (len(m.filteredResults)-1)/m.itemsPerPage + 1
		s.WriteString(fmt.Sprintf("Page %d of %d (Total: %d files)\n\n", m.page+1, totalPages, len(m.filteredResults)))

		// Display current page items
		startIdx := m.page * m.itemsPerPage
		endIdx := startIdx + m.itemsPerPage
		if endIdx > len(m.filteredResults) {
			endIdx = len(m.filteredResults)
		}

		// No results to display
		if len(m.filteredResults) == 0 {
			s.WriteString("No results to display\n")
		} else {
			// Display items
			for i := startIdx; i < endIdx; i++ {
				item := m.filteredResults[i]

				// Selected cursor
				cursor := " "
				if i-startIdx == m.cursor {
					cursor = ">"
				}

				// Check if this item is selected for download (only use the map as source of truth)
				selected := " "
				urlStr := item.url.String()

				// Check the map for selection state - it's the source of truth
				isSelected, exists := m.selectedFiles[urlStr]
				if exists && isSelected {
					// Use a clear checkmark symbol
					selected = "✓"
					// Only output debug for truly selected items
					fmt.Printf("DEBUG VIEW: Item %d ('%s') is SELECTED\n", i, item.name)

					// Also update the item.selected flag for consistency
					item.selected = true
				} else {
					// Ensure the item's selected flag is consistent with the map
					item.selected = false
					// Clear from map if it exists but is false (cleanup)
					if exists && !isSelected {
						delete(m.selectedFiles, urlStr)
					}
				}

				// File info with clearer selection formatting
				selectBox := "[ ]"
				if selected == "✓" {
					selectBox = "[✓]"
				}
				s.WriteString(fmt.Sprintf("%s %s %s (%d KB)\n",
					cursor, selectBox, item.name, item.size/1024))
			}
		}

		// Status message
		if m.status != "" {
			s.WriteString("\n" + m.status + "\n")
		}

		// Help text
		s.WriteString("\nSpace: Select | Enter: Download | j/k: Move | h/l: Pages | f: Filter | esc: Quit\n")

	case ModeFilter:
		s.WriteString("🔍 Filter Mode 🔍\n\n")
		s.WriteString(fmt.Sprintf("Filter terms: %s\n", m.filterInput.View()))
		if m.status != "" {
			s.WriteString("\n" + m.status + "\n")
		}

		// Show preview of filtered results
		if len(m.filteredResults) > 0 {
			s.WriteString(fmt.Sprintf("\nFound %d matching files\n", len(m.filteredResults)))
			// Show a few examples
			count := 3
			if len(m.filteredResults) < count {
				count = len(m.filteredResults)
			}
			for i := 0; i < count; i++ {
				s.WriteString(fmt.Sprintf("- %s\n", m.filteredResults[i].name))
			}
			if len(m.filteredResults) > count {
				s.WriteString("...\n")
			}
		} else {
			s.WriteString("\nNo files match the filter\n")
		}

		s.WriteString("\nEnter: Apply filter | esc: Cancel\n")

	case ModeDownloading:
		s.WriteString("⬇️  Downloading Mode ⬇️\n\n")

		// Current download info
		if m.downloadedSize > 0 {
			// Progress bar
			percent := 0.0
			if m.totalSize > 0 {
				percent = float64(m.downloadedSize) / float64(m.totalSize)
			}
			progressBar := m.progress.ViewAs(percent)

			s.WriteString(fmt.Sprintf("Downloading: %s\n", m.currentFile))
			s.WriteString(progressBar + "\n")
			s.WriteString(fmt.Sprintf("%d KB / %d KB (%.1f%%)\n\n",
				m.downloadedSize/1024,
				m.totalSize/1024,
				percent*100))
		} else if len(m.downloadQueue) > 0 {
			s.WriteString(fmt.Sprintf("Preparing to download %d files...\n", len(m.downloadQueue)))
			s.WriteString(m.spinner.View() + "\n\n")
		} else {
			s.WriteString("No active downloads\n\n")
		}

		// Download queue
		s.WriteString(fmt.Sprintf("Download Queue (%d):\n", len(m.downloadQueue)))
		if len(m.downloadQueue) == 0 {
			s.WriteString("Queue is empty\n")
		} else {
			for i, url := range m.downloadQueue {
				cursor := " "
				if i == m.queueCursor {
					cursor = ">"
				}

				// Get file name
				fileName := url.String()
				fileItem := findFileItemByURL(m.searchResults, url)
				if fileItem != nil {
					fileName = fileItem.name
				}

				s.WriteString(fmt.Sprintf("%s %s\n", cursor, fileName))
			}
		}

		// Status message
		if m.status != "" {
			s.WriteString("\n" + m.status + "\n")
		}

		// Help text
		s.WriteString("\nj/k: Navigate queue | d: Remove from queue | tab: Switch Mode | esc: Quit\n")
	}

	return s.String()
}

// downloadFile starts downloading a file and returns a tea.Cmd
func downloadFile(url *xdcc.IRCFile, outputPath string) tea.Cmd {
	return func() tea.Msg {
		// Log connection information for debugging
		fmt.Printf("Starting download: %s\n", url.String())

		// Set up a transfer
		transfer := xdcc.NewTransfer(xdcc.Config{
			File:    *url,
			OutPath: outputPath,
			SSLOnly: false,
		})

		// Start transfer
		err := transfer.Start()
		if err != nil {
			// Try to extract user-friendly error message
			userFriendlyError := err
			if strings.Contains(err.Error(), "queue is full") {
				userFriendlyError = fmt.Errorf("Bot's download queue is full. Try again later")
			} else if strings.Contains(err.Error(), "no slots open") {
				userFriendlyError = fmt.Errorf("Bot has no slots available. Try again later")
			} else if strings.Contains(err.Error(), "you must be on a known channel") {
				userFriendlyError = fmt.Errorf("Bot requires you to join its channel first")
			} else if strings.Contains(err.Error(), "banned") {
				userFriendlyError = fmt.Errorf("You are banned from this bot")
			}

			return errorMsg{
				err: userFriendlyError,
				url: url,
			}
		}

		// Set up a listener in a goroutine
		go func() {
			evts := transfer.PollEvents()
			for evt := range evts {
				// Process events (e.g., log progress)
				switch e := evt.(type) {
				case xdcc.TransferProgessEvent:
					fmt.Printf("Progress: %d bytes (%.2f KB/s)\n", e.TransferBytes, float64(e.TransferRate)/1024)
				case xdcc.TransferStartedEvent:
					fmt.Printf("Download started: %s (%.2f MB)\n", outputPath, float64(e.FileSize)/1024/1024)
				case xdcc.TransferCompletedEvent:
					fmt.Printf("Download completed: %s\n", outputPath)
				case xdcc.TransferAbortedEvent:
					fmt.Printf("Download aborted: %s\n", e.Error)
				}
			}
		}()

		// Return initial status with minimal info
		return downloadProgressMsg{
			bytesDownloaded: 0,
			totalBytes:      0,
			url:             url,
			speed:           0,
		}
	}
}
