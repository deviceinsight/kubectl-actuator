package cmd

import (
	"errors"
	"fmt"
)

// ErrInterrupted is returned when the user interrupts a running command; the
// process exits with code 130 (128 + SIGINT) like other Unix tools.
var ErrInterrupted = errors.New("interrupted")

// ExitCodeError carries a specific process exit code through cobra to
// Execute. Err may be nil for exits whose meaning is already on screen,
// such as health reporting a non-UP pod; those are returned with
// SilenceErrors set so no redundant error line is printed.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

func (e *ExitCodeError) Unwrap() error { return e.Err }
