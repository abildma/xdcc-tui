package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"xdcc-tui/search"
	xdcc "xdcc-tui/xdcc"
)

// UI constants
var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	rowEvenStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	rowOddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

// Messages used with Bubble Tea ------------------------------------------------

type searchResultsMsg struct {
	results []search.XdccFileInfo
	err     error
}

type downloadEventMsg struct {
	index int
	evt   xdcc.TransferEvent
	err   error
	done  bool
}

type errMsg struct{ error }

// Model -----------------------------------------------------------------------

// downloadState tracks simple progress data to show in the list.
// We only keep transferred bytes and total for a text-based progress display.

type downloadState struct {
	bytesTotal     uint64
	bytesCompleted uint64
	completed      bool
	speed          float64
	ch             <-chan xdcc.TransferEvent
}

type Model struct {
	// inputs
	searchInput textinput.Model
	filterInput textinput.Model

	// data
	results         []search.XdccFileInfo
	filteredResults []search.XdccFileInfo
	cursor          int
	selected        map[int]struct{}
	downloads       map[int]*downloadState

	page int

	// helpers
	aggregator *search.ProviderAggregator

	// ui feedback
	status string
	busy   bool

	searchDone bool
	filterMode bool

	currentView view
}

type view int

const (
	viewSearch view = iota
	viewDownloads
)

const pageSize = 20

func NewModel() Model {
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = "search keywords…"
	ti.CharLimit = 256
	ti.Width = 40

	fi := textinput.New()
	fi.Placeholder = "filter results (e.g., .mp4, >1GB)"
	fi.CharLimit = 100
	fi.Width = 40

	aggr := search.NewProviderAggregator(
		&search.XdccEuProvider{},
		&search.SunXdccProvider{},
	)

	return Model{
		searchInput: ti,
		filterInput: fi,
		selected:    make(map[int]struct{}),
		downloads:   make(map[int]*downloadState),
		aggregator:  aggr,
		status:      "Enter keywords and press <enter> to search | Tab: switch view | /: filter",
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// getCurrentResults returns the current results slice (filtered or unfiltered)
func (m *Model) getCurrentResults() []search.XdccFileInfo {
	if m == nil {
		return nil
	}
	if len(m.filteredResults) > 0 {
		return m.filteredResults
	}
	return m.results
}

func (m *Model) applyFilter() {
	filter := strings.TrimSpace(m.filterInput.Value())
	if filter == "" {
		m.filteredResults = nil
		m.cursor = 0
		m.page = 0
		m.status = "Filter cleared"
		return
	}

	var filtered []search.XdccFileInfo

	// Check for size filters (e.g., >1GB, <500MB)
	if filter[0] == '>' || filter[0] == '<' {
		// Parse size filter
		compareFunc := func(a, b int64) bool { return a > b }
		if filter[0] == '<' {
			compareFunc = func(a, b int64) bool { return a < b }
		}

		size, err := parseSizeFilter(strings.TrimSpace(filter[1:]))
		if err == nil {
			for _, r := range m.results {
				if compareFunc(r.Size, size) {
					filtered = append(filtered, r)
				}
			}
		}
	} else if strings.HasPrefix(filter, ".") {
		// File extension filter
		ext := strings.ToLower(filter)
		for _, r := range m.results {
			if strings.HasSuffix(strings.ToLower(r.Name), ext) {
				filtered = append(filtered, r)
			}
		}
	} else {
		// Simple filename filter (case insensitive)
		filterLower := strings.ToLower(filter)
		for _, r := range m.results {
			if strings.Contains(strings.ToLower(r.Name), filterLower) {
				filtered = append(filtered, r)
			}
		}
	}

	m.filteredResults = filtered
	m.cursor = 0
	m.page = 0
	m.status = fmt.Sprintf("Filter: %s (%d results)", filter, len(filtered))
}

func parseSizeFilter(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	// Find the numeric part
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] >= '0' && s[i] <= '9') {
		i++
	}

	if i == 0 {
		return 0, fmt.Errorf("no number found")
	}

	// Parse the number
	numberStr := s[:i]
	size, err := strconv.ParseFloat(numberStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %v", err)
	}

	// If there's no unit, assume bytes
	if i >= len(s) {
		return int64(size), nil
	}

	// Parse the unit
	unit := strings.TrimSpace(s[i:])
	switch {
	case strings.HasPrefix(unit, "k"):
		return int64(size * 1024), nil
	case strings.HasPrefix(unit, "m"):
		return int64(size * 1024 * 1024), nil
	case strings.HasPrefix(unit, "g"):
		return int64(size * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.busy {
			// ignore key events while a search is running
			return m, nil
		}

		if m.filterMode {
			// Handle Enter key in filter mode
			if msg.String() == "enter" {
				m.filterMode = false
				m.applyFilter()
				return m, nil
			}

			switch msg.String() {
			case "esc":
				m.filterMode = false
				m.filteredResults = nil
				m.status = "Filter cleared"
				m.cursor = 0
				m.page = 0
				return m, nil
			case "backspace":
				if m.filterInput.Value() == "" {
					m.filterMode = false
					m.filteredResults = nil
					m.status = "Filter cleared"
					return m, nil
				}
			}

			// Don't process the '/' key in filter mode
			if msg.String() == "/" {
				return m, nil
			}

			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		switch msg.String() {
		case "tab":
			if m.currentView == viewSearch {
				m.currentView = viewDownloads
			} else {
				m.currentView = viewSearch
			}
			return m, nil
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if !m.searchDone {
				// start search
				query := strings.TrimSpace(m.searchInput.Value())
				if query == "" {
					m.status = "please type something to search"
					return m, nil
				}
				m.searchDone = true
				m.results = nil
				m.filteredResults = nil
				m.cursor = 0
				m.page = 0
				m.busy = true
				m.status = "searching…"
				return m, tea.Batch(runSearchCmd(m.aggregator, strings.Split(query, " ")), textinput.Blink)
			}
			// search already done -> treat Enter as download key
			indices := m.indicesToDownload()
			if len(indices) == 0 {
				return m, nil
			}
			return m, m.startDownloads(indices)
		case "left", "h":
			if m.currentView == viewSearch && m.cursor > 0 {
				if m.cursor >= pageSize {
					m.cursor -= pageSize
				} else {
					m.cursor = 0
				}
				m.page = m.cursor / pageSize
			}
		case "right", "l":
			if m.currentView == viewSearch && m.cursor < len(m.results)-1 {
				if m.cursor+pageSize < len(m.results) {
					m.cursor += pageSize
				} else {
					m.cursor = len(m.results) - 1
				}
				m.page = m.cursor / pageSize
			}
		case "/":
			if m.currentView == viewSearch && m.searchDone && !m.filterMode {
				m.filterMode = true
				// Clear any existing filter text when starting a new filter
				m.filterInput.Reset()
				m.filterInput.Focus()
				m.status = "Filter: " + m.filterInput.Value()
				// Return here to prevent the '/' from being added to the input
				return m, nil
			}
		case "esc":
			if m.filterMode {
				m.filterMode = false
				m.status = "Filter cleared | " + m.status
				m.applyFilter()
			} else if m.searchDone {
				// Return to search input
				m.searchDone = false
				m.searchInput.Reset()
				m.searchInput.Focus()
				m.status = "Enter search query"
				m.results = nil
				m.filteredResults = nil
				m.cursor = 0
				m.page = 0
			}
		case "up", "k":
			if m.currentView != viewSearch || m.filterMode {
				break
			}
			if m.searchDone {
				results := m.getCurrentResults()
				if len(results) == 0 {
					break
				}
				if m.cursor > 0 {
					m.cursor--
				}
				if m.cursor < m.page*pageSize {
					m.page--
				}
			}
		case "down", "j":
			if m.currentView != viewSearch || m.filterMode {
				break
			}
			results := m.getCurrentResults()
			if len(results) == 0 {
				break
			}
			if m.cursor < len(results)-1 {
				m.cursor++
			}
			if m.cursor >= (m.page+1)*pageSize {
				m.page++
			}
		case " ": // spacebar
			if m.currentView != viewSearch {
				break
			}
			if len(m.results) == 0 {
				break
			}
			if _, ok := m.selected[m.cursor]; ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		case "d":
			if m.currentView != viewSearch {
				break
			}
			indices := m.indicesToDownload()
			if len(indices) == 0 {
				break
			}
			return m, m.startDownloads(indices)
		}
	case searchResultsMsg:
		m.busy = false
		m.searchDone = true
		m.searchInput.Blur()
		if msg.err != nil {
			m.status = fmt.Sprintf("search failed: %v", msg.err)
			return m, nil
		}
		// sort results by size descending for convenience
		sort.Slice(msg.results, func(i, j int) bool {
			return msg.results[i].Size > msg.results[j].Size
		})
		m.results = msg.results
		m.filteredResults = nil
		m.cursor = 0
		m.page = 0
		m.selected = make(map[int]struct{})
		m.status = fmt.Sprintf("found %d results | / to filter", len(msg.results))
	case downloadEventMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("download error: %v", msg.err)
			return m, nil
		}
		ds, ok := m.downloads[msg.index]
		if !ok {
			return m, nil
		}
		if msg.done {
			ds.completed = true
			m.status = fmt.Sprintf("✔ %s completed", m.results[msg.index].Name)
			return m, nil
		}
		switch e := msg.evt.(type) {
		case *xdcc.TransferStartedEvent:
			ds.bytesTotal = uint64(e.FileSize)
		case *xdcc.TransferProgessEvent:
			ds.bytesCompleted += e.TransferBytes
			ds.speed = float64(e.TransferRate)
		case *xdcc.TransferCompletedEvent:
			ds.completed = true
			msg.done = true
			m.status = fmt.Sprintf("✔ %s completed", m.results[msg.index].Name)
		}
		// schedule next poll if not done
		if !msg.done {
			return m, pollDownloadCmd(msg.index, ds.ch)
		}
	case errMsg:
		m.busy = false
		m.status = fmt.Sprintf("error: %v", msg)
	}

	// let textinput update regardless of state so user can type again after search
	var cmd tea.Cmd
	if m.filterMode {
		m.filterInput, cmd = m.filterInput.Update(msg)
	} else {
		m.searchInput, cmd = m.searchInput.Update(msg)
	}
	return m, cmd
}

// indicesToDownload returns selected indices or current cursor if none selected
func (m Model) indicesToDownload() []int {
	results := m.getCurrentResults()
	if len(results) == 0 {
		return nil
	}
	indices := make([]int, 0)
	if len(m.selected) == 0 {
		// If we're in filtered view, we need to map the filtered index back to the original results
		if len(m.filteredResults) > 0 && m.cursor < len(m.filteredResults) {
			// Find the index of the current filtered result in the original results
			for i, r := range m.results {
				if r.Name == m.filteredResults[m.cursor].Name && r.Size == m.filteredResults[m.cursor].Size {
					indices = append(indices, i)
					break
				}
			}
		} else if m.cursor < len(m.results) {
			indices = append(indices, m.cursor)
		}
	} else {
		for idx := range m.selected {
			indices = append(indices, idx)
		}
	}
	return indices
}

// helper to poll one event from channel
func pollDownloadCmd(index int, ch <-chan xdcc.TransferEvent) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return downloadEventMsg{index: index, done: true}
		}
		return downloadEventMsg{index: index, evt: evt}
	}
}

// startDownloads prepares downloadState and returns a Batch cmd
func (m *Model) startDownloads(indices []int) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(indices))
	for _, idx := range indices {
		file := m.results[idx]
		transfer := xdcc.NewTransfer(xdcc.Config{File: file.URL})
		// start connection (blocking until IRC connect attempt returns)
		if err := transfer.Start(); err != nil {
			cmds = append(cmds, func() tea.Msg { return downloadEventMsg{index: idx, err: err} })
			continue
		}
		ch := transfer.PollEvents()
		m.downloads[idx] = &downloadState{bytesTotal: uint64(file.Size), ch: ch}
		cmds = append(cmds, pollDownloadCmd(idx, ch))
	}
	m.status = fmt.Sprintf("started %d download(s)", len(indices))

	return tea.Batch(cmds...)
}

// View implements tea.Model
func (m Model) View() string {
	// Show filter input when in filter mode
	if m.filterMode {
		return fmt.Sprintf(
			"Filter: %s\n\n%s",
			m.filterInput.View(),
			"(esc to cancel, enter to apply | e.g., .mp4, >1GB, <500MB)",
		)
	}

	var b strings.Builder

	// Show search input when no search has been performed yet
	if !m.searchDone {
		return fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			titleStyle.Render("XDCC-TUI"),
			m.searchInput.View(),
			"(press Enter to search, Esc to exit)",
		)
	}

	b.WriteString(titleStyle.Render("XDCC-TUI") + "\n\n")
	if m.currentView == viewSearch {
		b.WriteString(m.searchInput.View() + "\n\n")

		// Get the current results (filtered or unfiltered)
		results := m.getCurrentResults()

		// header
		b.WriteString(headerStyle.Render(fmt.Sprintf("Page %d/%d | %-2s %-3s %-40s %8s %s",
			m.page+1,
			(len(results)+pageSize-1)/pageSize, // total pages
			"", "", "Name", "Size", "Pack")) + "\n")

		// results list
		start := m.page * pageSize
		end := start + pageSize
		if end > len(results) {
			end = len(results)
		}

		// Show message if no results
		if len(results) == 0 {
			b.WriteString("\n  No results found")
		}

		for i := start; i < end; i++ {
			res := results[i]

			cursor := "  "
			if i == m.cursor {
				cursor = cursorStyle.Render("> ")
			}
			sel := ""
			if _, ok := m.selected[i]; ok {
				sel = selectedStyle.Render("[x] ")
			} else {
				sel = "[ ] "
			}
			sizeStr := FormatSize(res.Size)
			ext := filepath.Ext(res.Name)
			nameWithoutExt := strings.TrimSuffix(res.Name, ext)
			var nameDisplay string
			if m.filterInput.Value() != "" && strings.HasPrefix(m.filterInput.Value(), ".") {
				nameDisplay = fmt.Sprintf("%s%s",
					nameWithoutExt,
					lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Render(ext))
			} else {
				nameDisplay = res.Name
			}

			// Show a simple server identifier
			serverInfo := ""
			if len(results) > 0 {
				serverInfo = fmt.Sprintf("Server %d", res.Slot%10) // Simple hash-like identifier
			}

			fileInfo := fmt.Sprintf("%s (%s) - %s",
				nameDisplay,
				FormatSize(res.Size),
				serverInfo,
			)
			line := fmt.Sprintf("%s%s%-40.40s %8s %s", cursor, sel, fileInfo, sizeStr, res.URL.String())
			// alternating row style for readability
			if i%2 == 0 {
				line = rowEvenStyle.Render(line)
			} else {
				line = rowOddStyle.Render(line)
			}
			if _, ok := m.selected[i]; ok {
				line = selectedStyle.Render(line)
			}
			if i == m.cursor {
				line = cursorStyle.Render(line)
			}
			b.WriteString(line + "\n")

		}
	} else {
		// downloads view
		b.WriteString(headerStyle.Render(fmt.Sprintf("%-40s %12s", "Name", "Progress")) + "\n")
		for idx, ds := range m.downloads {
			file := m.results[idx]
			prog := "pending"
			if ds.completed {
				prog = "✔ completed"
			} else if ds.bytesTotal > 0 {
				pct := float64(ds.bytesCompleted) / float64(ds.bytesTotal) * 100
				if pct < 0.1 {
					pct = 0.1
				}
				prog = fmt.Sprintf("%5.1f%% %5.1f MB/s", pct, ds.speed/float64(search.MegaByte))
			}
			line := fmt.Sprintf("%-40.40s %12s", file.Name, prog)
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render(m.status))

	return b.String()
}

// Helper commands ----------------------------------------------------------------

func runSearchCmd(aggr *search.ProviderAggregator, keywords []string) tea.Cmd {
	return func() tea.Msg {
		res, err := aggr.Search(keywords)
		return searchResultsMsg{results: res, err: err}
	}
}

// ---------------- utility copied from cmd/main.go -----------------------------

func FormatSize(size int64) string {
	if size < 0 {
		return "--"
	}

	if size >= search.GigaByte {
		return fmt.Sprintf("%.2fGB", float64(size)/float64(search.GigaByte))
	} else if size >= search.MegaByte {
		return fmt.Sprintf("%.2fMB", float64(size)/float64(search.MegaByte))
	} else if size >= search.KiloByte {
		return fmt.Sprintf("%.2fKB", float64(size)/float64(search.KiloByte))
	}
	return fmt.Sprintf("%dB", size)
}
