package tui

import (
	"fmt"
	stdlog "log"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonson/tsgrok/internal/funnel"
)

// createFunnelCmd calls the backend function to create a funnel
// and returns a message indicating success or failure.
func createFunnelCmd(name, target string, logger *stdlog.Logger) tea.Cmd {
	return func() tea.Msg {
		funnel, err := funnel.CreateEphemeralFunnel(name, target, logger)
		if err != nil {
			return funnelCreateErrMsg{err}
		}
		return funnelCreatedMsg{funnel}
	}
}

// deleteFunnelCmd finds a funnel by ID, calls its Destroy method,
// removes it from the registry, and returns a message.
func deleteFunnelCmd(id string, registry *funnel.FunnelRegistry) tea.Cmd {
	return func() tea.Msg {
		funnel, err := registry.GetFunnel(id)
		if err != nil {
			// Funnel not found in registry, maybe already deleted?
			// Consider logging this, but return success msg for idempotency?
			// Or return a specific error? For now, let's return an error.
			return funnelDeleteErrMsg{id: id, err: fmt.Errorf("funnel not found in registry: %w", err)}
		}

		if err := funnel.Destroy(); err != nil {
			// Attempted to destroy, but failed
			return funnelDeleteErrMsg{id: id, err: err}
		}

		// Destroy succeeded, now remove from registry
		registry.RemoveFunnel(id)

		return funnelDeletedMsg{id: id}
	}
}

// copyToClipboardCmd writes the given text to the system clipboard.
func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(text)
		if err != nil {
			// Log error or return an error message
			return clipboardWriteErrorMsg{err: err}
		} else {
			// Optionally return a success message
			return clipboardWriteSuccessMsg{}
		}
	}
}
