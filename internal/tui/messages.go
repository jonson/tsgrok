package tui

import (
	"fmt"

	"github.com/jonson/tsgrok/internal/funnel"
)

type funnelCreatedMsg struct {
	funnel funnel.Funnel
}

type funnelCreateErrMsg struct {
	err error
}

// Ensure funnelCreateErrMsg implements the error interface
func (e funnelCreateErrMsg) Error() string {
	return e.err.Error()
}

type funnelDeletedMsg struct {
	id string // ID of the funnel that was deleted
}

type funnelDeleteErrMsg struct {
	id  string // ID of the funnel that failed to delete
	err error
}

// Ensure funnelDeleteErrMsg implements the error interface
func (e funnelDeleteErrMsg) Error() string {
	return fmt.Sprintf("failed to delete funnel %s: %v", e.id, e.err)
}

type clipboardWriteSuccessMsg struct{}
type clipboardWriteErrorMsg struct{ err error }
type clearStatusMsg struct{}

// Ensure clipboardWriteErrorMsg implements the error interface
func (e clipboardWriteErrorMsg) Error() string {
	return fmt.Sprintf("clipboard error: %v", e.err)
}
