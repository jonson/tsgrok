package tui

import (
	"fmt"
	stdlog "log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonson/tsgrok/internal/funnel"
	"github.com/jonson/tsgrok/internal/util"
)

var (
	greenColor    = lipgloss.Color("36")
	subtleGrey    = lipgloss.Color("240")
	appInfoBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┬",
		TopRight:    "┤",
		BottomLeft:  "┴",
		BottomRight: "┘",
	}
	helpBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       " ",
		TopLeft:     "├",
		TopRight:    "─",
		BottomLeft:  "└",
		BottomRight: "─",
	}
	contentBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "*",
		Left:        "│",
		Right:       "│",
		TopRight:    "┐",
		TopLeft:     "┌",
		BottomRight: "*",
		BottomLeft:  "*",
	}

	// tabs
	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	tab = lipgloss.NewStyle().
		Border(tabBorder, true).
		BorderForeground(greenColor).
		Padding(0, 1)

	activeTab = tab.Border(activeTabBorder, true)

	tabGap = tab.
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false)
)

const helpContent = `
Keybindings:

Global:
  q / ctrl+c : Quit
  ?          : Show/Hide Help

List View:
  ↑/↓        : Navigate Funnels
  n          : New Funnel
  d          : Delete Selected Funnel
  c          : Copy Public URL of Selected Funnel
  enter      : View Funnel Details

Create View:
  tab        : Switch Input Fields
  enter      : Create Funnel
  esc        : Cancel Creation

Confirm Delete View:
  y          : Confirm Deletion
  n / esc    : Cancel Deletion

Detail View:
  tab / → / l: Next Tab
  shift+tab / ← / h: Previous Tab
  c          : Copy Public URL (Info Tab)
  enter      : View Request Details (Request Log Tab)
  esc    : Back to List View

Request Detail View:
  esc    : Back to Request Log
`

// viewState indicates which view is currently active
type viewState int

const (
	viewList          viewState = iota // Default view, will list funnels
	viewCreate                         // View for creating a new funnel
	viewConfirmDelete                  // View for confirming deletion
	viewDetail                         // View showing details for a selected funnel
	viewHelp                           // View displaying keybindings/help
	viewRequestDetail                  // View showing details of a specific proxied request
)

// --- Model ---

type model struct {
	width  int // Current terminal width
	height int // Current terminal height

	state viewState

	// State for viewCreate
	funnelNameInput   textinput.Model
	funnelTargetInput textinput.Model
	inputFocusIndex   int
	createErrMsg      string // To store creation errors
	isCreating        bool   // Flag to indicate creation is in progress

	// State for viewConfirmDelete
	deletingFunnelID string // ID of the funnel being confirmed for deletion
	spinner          spinner.Model

	// State for viewDetail
	detailedFunnelID string // ID of the funnel being viewed
	detailTabIndex   int    // 0 for Info, 1 for Requests

	// Status message state
	statusMessage string // Message to display temporarily
	tickerActive  bool   // Flag to track if the status clear timer is running

	funnelRegistry *funnel.FunnelRegistry
	previousState  viewState // To store the state before opening help

	// viewport for help view
	viewport viewport.Model

	// State for viewList
	table       table.Model
	funnelOrder []string // Slice of funnel IDs to maintain order matching table rows

	requestTable    table.Model
	selectedRequest *funnel.CaptureRequestResponse // The request being inspected in viewRequestDetail

	logger *stdlog.Logger
}

func InitialModel(funnelRegistry *funnel.FunnelRegistry, logger *stdlog.Logger) model {
	nameInput := textinput.New()
	nameInput.Placeholder = "my-funnel-name"
	nameInput.Focus()
	nameInput.CharLimit = 63 // Max length for hostnames/subdomains
	nameInput.Width = 30

	targetInput := textinput.New()
	targetInput.Placeholder = "http://localhost:8000"
	targetInput.CharLimit = 256
	targetInput.Width = 30

	// Initialize spinner
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(greenColor)
	sp.Spinner = spinner.Dot

	viewport := viewport.New(1, 1)
	viewport.SetContent(helpContent)

	return model{
		width:             0,        // Placeholder, actual width will be set later
		height:            0,        // Placeholder, actual height will be set later
		state:             viewList, // Start in list view
		funnelNameInput:   nameInput,
		funnelTargetInput: targetInput,
		inputFocusIndex:   0, // Focus name input first
		funnelRegistry:    funnelRegistry,
		table:             createInitialTable(), // Call helper to create the table
		requestTable:      createRequestTable(), // Call helper to create the table
		funnelOrder:       []string{},           // Initialize empty order slice
		spinner:           sp,                   // Add initialized spinner
		isCreating:        false,                // Initialize isCreating flag
		viewport:          viewport,
		logger:            logger,
	}
}

// Helper function to create the initial table model
func createInitialTable() table.Model {
	columns := []table.Column{
		{Title: "Name"},         // Removed fixed width
		{Title: "Local Target"}, // Removed fixed width
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}), // Start with no rows
		table.WithFocused(true),       // Focus the table by default in this view
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(subtleGrey)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(greenColor).Bold(false)
	t.SetStyles(s)

	return t
}

func createRequestTable() table.Model {
	columns := []table.Column{
		{Title: "Timestamp"},
		{Title: "Method"},
		{Title: "Status"},
		{Title: "URL"},
		{Title: "ID"}, // Hidden column for request ID
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(subtleGrey)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(greenColor).Bold(false)
	t.SetStyles(s)

	return t
}

func (m model) Init() tea.Cmd {
	return textinput.Blink // Start the cursor blinking
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keybindings first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Special case: if in help view, 'q' should go back, not quit entirely
			if m.state == viewHelp {
				m.state = m.previousState
				return m, nil
			}
			// Prevent quitting if in create view AND an input is focused
			if m.state == viewCreate && (m.funnelNameInput.Focused() || m.funnelTargetInput.Focused()) {
				// Let the input handler process 'q'
				break // Fall through to view-specific handlers
			}
			// Prevent quitting globally if in request detail view (let view handler decide)
			if m.state == viewRequestDetail && msg.String() == "q" {
				break // Fall through to view-specific handlers (which will ignore 'q')
			}
			return m, tea.Quit
		case "?":
			// Don't open help from help view or create view
			if m.state != viewHelp && m.state != viewCreate {
				m.previousState = m.state
				m.state = viewHelp
				// Ensure focused elements are blurred when entering help
				m.table.Blur()
				m.funnelNameInput.Blur()
				m.funnelTargetInput.Blur()
				// Potentially add blurring for other focusable elements if added later
				return m, nil
			}
		}

	// Handle funnel creation results globally, regardless of view
	case funnelCreatedMsg:
		m.funnelRegistry.AddFunnel(msg.funnel)
		// Rebuild the table rows AND the funnelOrder slice from the registry
		rows := []table.Row{}
		newOrder := []string{}
		// Note: Iterating over map doesn't guarantee order, but table handles selection separately.
		// We build the order slice *as we build the rows* to ensure they match.
		for id, funnel := range m.funnelRegistry.Funnels {
			newOrder = append(newOrder, id) // Add ID to the order list
			rows = append(rows, table.Row{
				funnel.Name(),        // Column 1: Name (assuming funnel.Name() exists)
				funnel.LocalTarget(), // Column 2: Local Target
			})
		}
		m.funnelOrder = newOrder // Update the model's order slice
		m.table.SetRows(rows)

		m.state = viewList   // Switch back to list view on success
		m.createErrMsg = ""  // Clear any previous error
		m.isCreating = false // Ensure creating flag is reset
		m.funnelNameInput.Blur()
		m.funnelTargetInput.Blur()
		m.table.Focus()
		return m, nil // No command needed from focusing

	case funnelCreateErrMsg:
		m.createErrMsg = msg.Error() // Store the error message
		m.isCreating = false         // Ensure creating flag is reset
		// Stay in the create view so the user can see the error
		// Ensure the correct input is focused if we were in create view
		if m.state == viewCreate {
			cmds := []tea.Cmd{}
			if m.inputFocusIndex == 0 {
				cmds = append(cmds, m.funnelNameInput.Focus())
				m.funnelTargetInput.Blur()
			} else {
				m.funnelNameInput.Blur()
				cmds = append(cmds, m.funnelTargetInput.Focus())
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil // No further command needed right now

	case funnelDeletedMsg:
		// Rebuild table rows and funnelOrder after successful deletion
		rows := []table.Row{}
		newOrder := []string{}
		for id, funnel := range m.funnelRegistry.Funnels {
			newOrder = append(newOrder, id)
			rows = append(rows, table.Row{
				funnel.Name(),
				funnel.LocalTarget(),
			})
		}
		m.funnelOrder = newOrder // Update the order slice
		m.table.SetRows(rows)
		// Ensure cursor is valid after deletion
		if m.table.Cursor() >= len(rows) && len(rows) > 0 {
			m.table.SetCursor(len(rows) - 1)
		} else if len(rows) == 0 {
			m.table.SetCursor(0)
		}
		// Refocus the table
		m.table.Focus()
		return m, nil // No command needed from focusing

	case funnelDeleteErrMsg:
		// TODO: Display the delete error message (e.g., in a status bar/footer)
		// For now, just log it or ignore, as we're already back in list view
		fmt.Fprintf(os.Stderr, "Error deleting funnel: %v\n", msg)
		return m, nil

	// Add a case to handle incoming proxied requests
	case funnel.ProxyRequestMsg:
		// If we are currently viewing the request log for any funnel, refresh it.
		if m.state == viewDetail && m.detailTabIndex == 1 {
			m.populateRequestTable() // Re-populate with latest requests
			// We don't need to return a command here, just update the model state.
		}
		return m, nil // No command needed after processing the request msg

	// Handle Clipboard Messages & Status Clearing
	case clipboardWriteSuccessMsg:
		m.statusMessage = "URL Copied!"
		m.tickerActive = true
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})

	case clipboardWriteErrorMsg:
		m.statusMessage = msg.Error()
		m.tickerActive = true
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { // Show errors longer
			return clearStatusMsg{}
		})

	case clearStatusMsg:
		m.statusMessage = ""
		m.tickerActive = false
		return m, nil

	case tea.WindowSizeMsg:
		// Update model dimensions
		m.width = msg.Width
		m.height = msg.Height

		// Calculate heights (leaving space for header/footer)
		headerHeight := 3                               // Approximate height for the view header/title
		footerHeight := lipgloss.Height(m.footerView()) // Use actual footer height
		verticalMarginHeight := headerHeight + footerHeight

		// Set Table Dimensions
		tableHeight := m.height - verticalMarginHeight
		tableWidth := m.width - 4 // border + margin on each side
		m.table.SetHeight(tableHeight)
		m.table.SetWidth(tableWidth)
		m.requestTable.SetHeight(tableHeight)
		m.requestTable.SetWidth(tableWidth)

		totalWidth := tableWidth // Basic adjustment for potential borders
		if totalWidth < 0 {
			totalWidth = 0
		}
		nameWidth := int(float64(totalWidth) * 0.3) // 30% for Name
		targetWidth := totalWidth - nameWidth - 4   // for some reason we need the extra 4

		// Create new column definitions with calculated widths
		newColumns := []table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Local Target", Width: targetWidth},
		}
		m.table.SetColumns(newColumns)

		urlWidth := tableWidth - 12 - 8 - 8 - 20

		requestColumns := []table.Column{
			{Title: "Timestamp", Width: 12},
			{Title: "Method", Width: 8},
			{Title: "Status", Width: 8},
			{Title: "URL", Width: urlWidth},
			{}, // Hidden column, no title or width needed
		}
		m.requestTable.SetColumns(requestColumns)

		return m, nil
	}

	// Handle view-specific logic
	switch m.state {
	case viewList:
		return m.updateListView(msg)
	case viewCreate:
		return m.updateCreateView(msg)
	case viewConfirmDelete:
		return m.updateConfirmDeleteView(msg)
	case viewDetail:
		return m.updateDetailView(msg)
	case viewHelp:
		return m.updateHelpView(msg) // Add call to new update function
	case viewRequestDetail:
		return m.updateRequestDetailView(msg)
	}

	return m, nil
}

// updateListView handles updates when the list view is active.
func (m model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n": // Switch to create view
			m.state = viewCreate
			// Reset fields and focus
			m.funnelNameInput.Reset()
			m.funnelTargetInput.Reset()
			m.inputFocusIndex = 0
			m.funnelNameInput.Focus()
			m.funnelTargetInput.Blur()
			m.createErrMsg = ""
			// Make sure table loses focus when switching away
			m.table.Blur()
			return m, textinput.Blink

		case "d": // Initiate deletion
			// Ensure there are rows and one is selected
			selectedIndex := m.table.Cursor()
			if selectedIndex >= 0 && selectedIndex < len(m.funnelOrder) {
				m.deletingFunnelID = m.funnelOrder[selectedIndex] // Get ID from the order slice
				m.state = viewConfirmDelete
				m.table.Blur() // Unfocus table while confirming
				return m, nil  // Don't pass 'd' to table
			}
			// If no rows or index out of bounds, do nothing
			return m, nil

		case "c": // Copy public URL
			selectedIndex := m.table.Cursor()
			if selectedIndex >= 0 && selectedIndex < len(m.funnelOrder) {
				funnelID := m.funnelOrder[selectedIndex]
				funnel, err := m.funnelRegistry.GetFunnel(funnelID)
				if err == nil {
					remoteURL := funnel.RemoteTarget()
					return m, copyToClipboardCmd(remoteURL)
				} // else: funnel not found? log error? do nothing?
			}
			return m, nil // Do nothing if index invalid or funnel lookup fails

		case "enter", " ": // View details
			selectedIndex := m.table.Cursor()
			if selectedIndex >= 0 && selectedIndex < len(m.funnelOrder) {
				m.detailedFunnelID = m.funnelOrder[selectedIndex]
				m.state = viewDetail
				m.detailTabIndex = 0 // Default to Info tab
				m.table.Blur()       // Unfocus table
				// Populate the request table immediately upon entering detail view
				m.populateRequestTable()
				return m, nil
			}
			return m, nil // Do nothing if index invalid

		// Any other key press is potentially for the table
		default:
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	}
	// Also pass other message types (like window resize) to the table
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// updateCreateView handles updates when the create view is active.
func (m model) updateCreateView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd // Use a slice to gather commands

	// If creating, only handle spinner ticks
	if m.isCreating {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		default:
			// Ignore other messages (like key presses) while creating
			return m, nil
		}
	}

	// Handle regular input if not creating
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// First, check for control keys (esc, tab, enter, arrows). If it's not one of them,
		// pass it directly to the input field update logic below.
		switch msg.Type {
		case tea.KeyEsc:
			m.state = viewList
			m.funnelNameInput.Blur()
			m.funnelTargetInput.Blur()
			m.createErrMsg = ""
			m.table.Focus() // Focus table when going back
			return m, nil   // No command needed

		case tea.KeyTab, tea.KeyUp, tea.KeyDown, tea.KeyShiftTab:
			// Don't switch focus if we are about to submit
			if m.inputFocusIndex == 1 && (msg.Type == tea.KeyDown || msg.Type == tea.KeyTab) {
				break // Let enter handle submission
			}

			isUp := msg.Type == tea.KeyUp || msg.Type == tea.KeyShiftTab || (msg.Type == tea.KeyTab && msg.Alt)

			// Cycle focus
			if isUp {
				m.inputFocusIndex--
			} else {
				m.inputFocusIndex++
			}

			// Wrap focus
			if m.inputFocusIndex > 1 {
				m.inputFocusIndex = 0
			}
			if m.inputFocusIndex < 0 {
				m.inputFocusIndex = 1
			}

			if m.inputFocusIndex == 0 {
				cmds = append(cmds, m.funnelNameInput.Focus())
				m.funnelTargetInput.Blur()
			} else {
				m.funnelNameInput.Blur()
				cmds = append(cmds, m.funnelTargetInput.Focus())
			}
			// Don't process the key further; return here
			return m, tea.Batch(cmds...)

		case tea.KeyEnter:
			// Only submit if the target input is focused
			if m.inputFocusIndex == 1 {
				funnelName := m.funnelNameInput.Value()
				funnelTarget := m.funnelTargetInput.Value()
				// TODO: Add input validation here
				m.isCreating = true
				m.createErrMsg = ""
				m.funnelNameInput.Blur()
				m.funnelTargetInput.Blur()
				cmds = append(cmds, createFunnelCmd(funnelName, funnelTarget, m.logger))
				cmds = append(cmds, m.spinner.Tick) // Start the spinner
				return m, tea.Batch(cmds...)
			} else {
				// If enter is pressed on the first input, move focus to the second
				m.inputFocusIndex = 1
				m.funnelNameInput.Blur()
				cmds = append(cmds, m.funnelTargetInput.Focus())
				return m, tea.Batch(cmds...)
			}
			// If not submitting, fall through to let the input handle the key if needed (though usually not for Enter)
			// break // prevent fallthrough to input update
		}
		// If the key was not a specific control key handled above, let the input field process it.
		// (This includes runes, space, etc.)

	default:
		// Handle other message types (like window resize) if necessary, though usually not needed here.
		// We don't want to pass non-key messages to the text inputs.
		return m, nil
	}

	// Handle character input for the focused field
	// Update the corresponding text input model and store the command
	var inputCmd tea.Cmd
	if m.inputFocusIndex == 0 {
		m.funnelNameInput, inputCmd = m.funnelNameInput.Update(msg)
	} else {
		m.funnelTargetInput, inputCmd = m.funnelTargetInput.Update(msg)
	}

	return m, inputCmd // Return the command from the input update
}

// updateConfirmDeleteView handles updates when the confirmation view is active.
func (m model) updateConfirmDeleteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y": // Confirm deletion
			// TODO: Trigger deletion command
			// For now, just switch back and clear the ID
			m.state = viewList
			m.table.Focus() // Refocus table
			funnelIDToDelete := m.deletingFunnelID
			m.deletingFunnelID = "" // Clear immediately
			// Return the delete command
			return m, deleteFunnelCmd(funnelIDToDelete, m.funnelRegistry)

		case "n", "N", "esc": // Cancel deletion
			m.state = viewList
			m.deletingFunnelID = ""
			m.table.Focus() // Refocus table
			return m, nil
		}
	}
	return m, nil
}

// updateDetailView handles updates when the detail view is active.
func (m model) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd // Declare cmd here to potentially capture it from table update

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Navigation
		case "esc", "backspace", "q": // Go back to list view
			m.state = viewList
			m.detailedFunnelID = "" // Clear the viewed funnel ID
			m.table.Focus()         // Refocus the main list table
			m.requestTable.Blur()   // Ensure request table is blurred
			return m, nil

		case "tab", "right", "l": // Switch to next tab
			previousTabIndex := m.detailTabIndex
			m.detailTabIndex = (m.detailTabIndex + 1) % 2 // Cycle through 2 tabs (0, 1)
			// If we just switched TO the request log tab, populate it and focus table
			if m.detailTabIndex == 1 {
				m.populateRequestTable()
				m.requestTable.Focus() // Focus the request table (doesn't return cmd)
				return m, nil          // Return nil cmd here
			} else if previousTabIndex == 1 { // Switched AWAY from request log
				m.requestTable.Blur()
			}
			return m, nil

		case "shift+tab", "left", "h": // Switch to previous tab
			previousTabIndex := m.detailTabIndex
			m.detailTabIndex = (m.detailTabIndex - 1 + 2) % 2 // Cycle through 2 tabs (0, 1)
			// If we just switched TO the request log tab, populate it and focus table
			if m.detailTabIndex == 1 {
				m.populateRequestTable()
				m.requestTable.Focus() // Focus the request table (doesn't return cmd)
				return m, nil          // Return nil cmd here
			} else if previousTabIndex == 1 { // Switched AWAY from request log
				m.requestTable.Blur()
			}
			return m, nil

		case "c": // Copy public URL (only on info tab)
			if m.detailTabIndex == 0 {
				funnel, err := m.funnelRegistry.GetFunnel(m.detailedFunnelID)
				if err == nil {
					remoteURL := funnel.RemoteTarget()
					return m, copyToClipboardCmd(remoteURL)
				} // else: funnel not found? log error?
			}
			return m, nil // Do nothing if not on info tab or error

		case "enter":
			if m.detailTabIndex == 1 {
				selectedRow := m.requestTable.SelectedRow()
				if len(selectedRow) < 5 { // Ensure row and ID exist (index 4)
					return m, nil // Or handle error
				}
				selectedRequestID := selectedRow[4] // Get ID from the hidden column

				funnel, err := m.funnelRegistry.GetFunnel(m.detailedFunnelID)
				if err == nil {
					// Find the request by ID in the linked list
					node := funnel.Requests.Head
					found := false
					for node != nil {
						if node.Request.ID == selectedRequestID {
							m.selectedRequest = &node.Request
							m.state = viewRequestDetail
							m.requestTable.Blur() // Unfocus table when leaving
							found = true
							break
						}
						node = node.Next
					}

					if found {
						return m, nil
					}
				}
			}
			return m, nil
		}
	}

	// If the request log tab is active, pass messages (like arrow keys) to the table.
	if m.state == viewDetail && m.detailTabIndex == 1 {
		m.requestTable, cmd = m.requestTable.Update(msg)
		return m, cmd
	}

	// Default: return model without command if no specific action taken
	// or if not in the request log tab
	return m, nil
}

// updateHelpView handles updates when the help view is active.
func (m model) updateHelpView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?": // Allow esc, q, or ? to close help
			m.state = m.previousState
			// TODO: Restore focus to the correct element based on previousState?
			// For now, just focus the table if returning to list view.
			if m.previousState == viewList {
				m.table.Focus()
			}
			return m, nil
		}
	}
	updatedViewport, cmd := m.viewport.Update(msg)
	m.viewport = updatedViewport
	return m, cmd
}

// updateRequestDetailView handles updates when the request detail view is active.
func (m model) updateRequestDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc": // Only Esc goes back to detail view (request log tab)
			m.state = viewDetail
			m.detailTabIndex = 1    // Ensure we return to the request log tab
			m.selectedRequest = nil // Clear the selected request
			m.requestTable.Focus()  // Refocus the request table
			return m, nil
		}
	}
	// Handle other message types (like window resize) if needed.
	// Currently, no other actions are handled in this basic version.
	return m, nil
}

// View renders the TUI's UI. It's called after every Update.
func (m model) View() string {
	// Footer (Render first to get its height)
	footer := m.footerView()
	footerHeight := lipgloss.Height(footer)

	// Main Content Area
	var mainContent string
	// Calculate height available for content: total height - footer height
	contentHeight := m.height - footerHeight
	if contentHeight < 0 {
		contentHeight = 0 // Prevent negative height
	}

	// Render view based on state, passing available height
	switch m.state {
	case viewList:
		mainContent = m.viewListView(contentHeight)
	case viewCreate:
		mainContent = m.viewCreateView(contentHeight)
	case viewConfirmDelete:
		mainContent = m.viewConfirmDeleteView(contentHeight)
	case viewDetail:
		mainContent = m.viewDetailView(contentHeight)
	case viewHelp:
		mainContent = m.viewHelpView(contentHeight)
	case viewRequestDetail:
		mainContent = m.viewRequestDetailView(contentHeight)
	}

	finalView := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Height(contentHeight).Render(mainContent), // Render content within allocated space
		footer,
	)

	// Ensure the total view doesn't exceed the terminal height
	// This might truncate if content is too large, refine later.
	return lipgloss.NewStyle().MaxHeight(m.height).Render(finalView)
}

func (m model) buildTopBorder(title string) string {
	titleStyled := lipgloss.NewStyle().Bold(true).Render(title)
	prefix := lipgloss.NewStyle().Foreground(greenColor).Render("┌─")
	suffix := ""
	for i := 0; i < m.width-lipgloss.Width(titleStyled)-lipgloss.Width(prefix)-1; i++ {
		suffix += "─"
	}
	suffix += "┐"
	suffix = lipgloss.NewStyle().Foreground(greenColor).Render(suffix)
	return lipgloss.JoinHorizontal(lipgloss.Left, prefix, titleStyled, suffix)
}

func (m model) renderContent(title string, content string, contentHeight int, horizontalPadding int) string {

	topBorder := m.buildTopBorder(title)
	contentWithBorder := lipgloss.NewStyle().
		Border(contentBorder, false, true, false, true).
		Padding(1, horizontalPadding).
		BorderForeground(greenColor).
		Width(m.width - 2).
		Height(contentHeight - 1).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, topBorder, contentWithBorder)
}

func (m model) viewListView(contentHeight int) string {
	const title = "Active Funnels"
	if len(m.table.Rows()) == 0 {
		// empty state
		emptyStyle := lipgloss.NewStyle().
			Italic(true).
			Foreground(subtleGrey). // Subtle grey color
			MarginLeft(2)           // Indent slightly
		emptyMessage := emptyStyle.Render("No active funnels. Press 'n' to create one.")

		return m.renderContent(title, emptyMessage, contentHeight, 1)
	}

	// render the table, we need to set the height and width to take into account the border
	m.table.SetHeight(contentHeight - 4)

	m.table.SetWidth(m.width - 20)

	return m.renderContent(title, m.table.View(), contentHeight, 1)
}

// viewCreateView renders the funnel creation form
func (m model) viewCreateView(contentHeight int) string {

	title := "Create New Funnel"
	nameInputView := m.funnelNameInput.View()
	targetInputView := m.funnelTargetInput.View()

	// Style for contextual help text (like the empty list view)
	helpTextStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(subtleGrey). // Subtle grey color
		PaddingLeft(0).         // No extra indent needed here usually
		PaddingTop(1).
		Width(m.width - 4) // Set width for wrapping (adjust padding as needed)

	// Determine contextual help text based on focus
	var helpText string
	if m.inputFocusIndex == 0 {
		helpText = "The tailscale node name for your funnel. Tailscale will automatically convert it to a dns-safe version, and will append a suffix if the name is already taken in your tailnet."
	} else {
		helpText = "The local HTTP server address to forward traffic to.  Examples are:\n8000\nlocalhost:8000\nhttp://localhost:8000\nhttps://localhost:8000  (for local HTTPS)\nhttps+insecure://localhost:8000  (for local HTTPS with self-signed cert)"
	}
	renderedHelpText := helpTextStyle.Render(helpText)

	// Display spinner or error message
	var statusOrErrorView string
	if m.isCreating {
		spinnerStyle := lipgloss.NewStyle().PaddingTop(1) // Add some space above the spinner
		statusOrErrorView = spinnerStyle.Render(fmt.Sprintf("%s Creating funnel...", m.spinner.View()))
	} else if m.createErrMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).PaddingTop(1) // Red, add space
		statusOrErrorView = errorStyle.Render("Error: " + m.createErrMsg)
	}

	// Combine the parts vertically
	content := lipgloss.JoinVertical(lipgloss.Left,
		nameInputView,
		targetInputView,
		renderedHelpText,  // Add the contextual help text here
		statusOrErrorView, // Spinner or error message
	)

	return m.renderContent(title, content, contentHeight, 1)
}

// viewConfirmDeleteView renders the deletion confirmation prompt.
func (m model) viewConfirmDeleteView(contentHeight int) string {
	funnelName := m.deletingFunnelID
	funnel, err := m.funnelRegistry.GetFunnel(m.deletingFunnelID)
	if err == nil {
		funnelName = funnel.Name()
	}

	question := fmt.Sprintf("Are you sure you want to delete funnel '%s'?", funnelName)

	content := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("9")).
		Padding(1, 2).
		Width(m.width-6).
		Align(lipgloss.Center, lipgloss.Center).
		Render(question + "\n\n(y/N)")

	return m.renderContent("", content, contentHeight, 1)
}

// viewRequestLogView just renders the table. Population happens in Update.
func (m model) viewRequestLogView(availableHeight int) string {
	// Rows are now populated by populateRequestTable called from Update
	m.requestTable.SetHeight(availableHeight - 2)
	return m.requestTable.View()
}

// populateRequestTable fetches requests for the current detailed funnel and updates the request table rows.
// It's called when entering detail view or switching to the request log tab.
func (m *model) populateRequestTable() {
	if m.detailedFunnelID == "" {
		m.requestTable.SetRows([]table.Row{}) // Clear table if no funnel selected
		return
	}

	funnel, err := m.funnelRegistry.GetFunnel(m.detailedFunnelID)
	if err != nil {
		// TODO: Maybe log this error? For now, clear the table.
		fmt.Fprintf(os.Stderr, "Error getting funnel %s for request table: %v\n", m.detailedFunnelID, err)
		m.requestTable.SetRows([]table.Row{})
		return
	}

	rows := []table.Row{}
	node := funnel.Requests.Head
	for node != nil {
		rows = append(rows, table.Row{
			node.Request.Timestamp.Format("15:04:05"),
			node.Request.Method(),
			strconv.Itoa(node.Request.StatusCode()),
			node.Request.Path(),
			node.Request.ID,
		})
		node = node.Next
	}
	m.requestTable.SetRows(rows)
}

func (m model) viewDetailView(availableHeight int) string {
	// shouldn't happen... we should have a better error message tho?
	funnel, err := m.funnelRegistry.GetFunnel(m.detailedFunnelID)
	if err != nil {
		// Handle error - funnel not found (shouldn't happen if ID is valid)
		return fmt.Sprintf("Error: Funnel %s not found.", m.detailedFunnelID)
	}

	var row string
	if m.detailTabIndex == 0 {
		row = lipgloss.JoinHorizontal(
			lipgloss.Top,
			activeTab.Render("Info"),
			tab.Render("Request Log"),
		)

	} else {
		row = lipgloss.JoinHorizontal(
			lipgloss.Top,
			tab.Render("Info"),
			activeTab.Render("Request Log"),
		)
	}

	// the filler is a neat trick, it has a bottom border and we just put in spaces to match the tab setup
	beforeFiller := tabGap.Render(" ")
	afterFiller := tabGap.Render(strings.Repeat(" ", max(0, m.width-lipgloss.Width(row)-7)))
	row = lipgloss.JoinHorizontal(lipgloss.Bottom, beforeFiller, row, afterFiller)

	tabContentHeight := availableHeight - lipgloss.Height(row) - 1

	// Content area
	var tabContent string
	switch m.detailTabIndex {
	case 0: // Info Tab
		// todo: move to a view function
		infoContent := fmt.Sprintf(
			"Name:         %s\nLocal Target: %s\nPublic URL:   %s",
			funnel.Name(), funnel.LocalTarget(), funnel.RemoteTarget(),
		)
		tabContent = infoContent
	case 1: // Requests Tab
		tabContent = m.viewRequestLogView(tabContentHeight)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, row, tabContent)
	return m.renderContent("Funnel Details", content, availableHeight, 0)
}

// footerView renders the footer/status bar
func (m model) footerView() string {
	footerText := fmt.Sprintf("%s v%s", util.ProgramName, util.ProgramVersion)

	// Define core help keys for each view
	var coreHelp string
	switch m.state {
	case viewList:
		if len(m.table.Rows()) == 0 {
			coreHelp = "n: new, q: quit, ?: help"
		} else {
			// coreHelp = "↑/↓: sel, n: new, d: del, c: copy, enter: details, q: quit, ?: help"
			coreHelp = "n: new, d: del, q: quit, ?: help"
		}
	case viewCreate:
		coreHelp = "tab: switch, enter: create, esc: cancel, ?: help"
	case viewConfirmDelete:
		coreHelp = "y: confirm, n/esc: cancel, ?: help"
	case viewDetail:
		coreHelp = "tab/←/→: tabs, c: copy, esc/q: back, ?: help"
	case viewHelp: // No specific help needed when already viewing help
		coreHelp = "esc/q: back, ←/→: scroll"
	case viewRequestDetail:
		coreHelp = "esc/q: back"
	}

	// Combine status message and help text
	statusOrHelp := coreHelp // Default to core help text
	if m.statusMessage != "" {
		statusOrHelp = m.statusMessage // Show status message if present
	}

	// Define styles for the footer sections
	borderColor := lipgloss.Color("36") // Cyan-like color
	// bgColor := lipgloss.Color("#222")
	fgColor := lipgloss.Color("#eee")

	statusHelpStyleBase := lipgloss.NewStyle().
		Foreground(fgColor).
		Padding(0, 1).
		Border(helpBorder, true).
		BorderForeground(borderColor)

	appInfoStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Padding(0, 1).
		Border(appInfoBorder, true).
		BorderForeground(borderColor)

	// --- Right Section (App Info) ---
	// Render *once* without fixed width to measure it accurately AND to use this render later
	appInfoRendered := appInfoStyle.Render(footerText)
	appInfoWidth := lipgloss.Width(appInfoRendered) // Measure it

	// --- Left Section (Status/Help) ---
	// Calculate remaining width
	statusHelpWidth := m.width - appInfoWidth - 2
	if statusHelpWidth < 0 {
		statusHelpWidth = 0
	}
	statusHelpStyle := statusHelpStyleBase.Width(statusHelpWidth)
	statusHelpRendered := statusHelpStyle.Render(statusOrHelp)

	// Combine the sections horizontally: Status/Help (Left) + App Info (Right)
	// Use the appInfoRendered that we measured directly.
	finalFooter := lipgloss.JoinHorizontal(lipgloss.Top, statusHelpRendered, appInfoRendered)

	return finalFooter
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// viewHelpView renders the keybindings help screen.
func (m model) viewHelpView(contentHeight int) string {
	m.viewport.Width = (m.width - 2)
	m.viewport.Height = (contentHeight - 3)

	return m.renderContent("Help", m.viewport.View(), contentHeight, 1)
}

// viewRequestDetailView renders the details of a selected HTTP request.
func (m model) viewRequestDetailView(contentHeight int) string {
	title := "Request Details"
	if m.selectedRequest == nil {
		return m.renderContent(title, "Error: No request selected.", contentHeight, 1)
	}

	requestInfo := fmt.Sprintf(
		"URL:    %s\nMethod: %s\nStatus: %d",
		m.selectedRequest.Path(),
		m.selectedRequest.Method(),
		m.selectedRequest.StatusCode(),
	)

	formatHeaders := func(headers map[string]string) string {
		var builder strings.Builder
		if len(headers) == 0 {
			builder.WriteString("  (No headers)")
		} else {
			// Get keys and sort them
			keys := make([]string, 0, len(headers))
			for k := range headers {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			// Iterate over sorted keys
			for _, k := range keys {
				v := headers[k]
				builder.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
			// Remove trailing newline
			result := builder.String()
			return strings.TrimSuffix(result, "\n")
		}
		return builder.String()
	}

	responseHeadersTitle := lipgloss.NewStyle().Bold(true).Render("Response Headers")
	responseHeadersContent := formatHeaders(m.selectedRequest.Response.Headers)

	requestHeadersTitle := lipgloss.NewStyle().Bold(true).Render("Request Headers")
	requestHeadersContent := formatHeaders(m.selectedRequest.Request.Headers)

	// Simple vertical layout for now
	content := lipgloss.JoinVertical(lipgloss.Left,
		requestInfo,
		"\n", // Spacer
		responseHeadersTitle,
		responseHeadersContent,
		"\n", // Spacer
		requestHeadersTitle,
		requestHeadersContent,
	)

	// Use the standard renderContent helper
	return m.renderContent(title, content, contentHeight, 1)
}
