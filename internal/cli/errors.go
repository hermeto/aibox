// Package cli builds the `aibox` command tree.
package cli

import (
	"errors"
	"strconv"
	"strings"
)

// Exit codes (see specs/002-cli-skeleton/contracts/cli.md).
const (
	ExitOK             = 0
	ExitFailure        = 1
	ExitNotImplemented = 2
	ExitUsage          = 64
)

// ExitError carries an exit code. Silent errors were already reported to the user
// (e.g. a failing doctor report) and are not printed again.
type ExitError struct {
	Code   int
	Err    error
	Silent bool
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return "exit " + strconv.Itoa(e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// UsageError wraps err as a usage error (exit 64).
func UsageError(err error) error { return &ExitError{Code: ExitUsage, Err: err} }

// ExitCode maps an error returned by the command tree to a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	msg := err.Error()
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand flag", "flag needs an argument", "invalid argument", "accepts ", "requires "} {
		if strings.HasPrefix(msg, p) {
			return ExitUsage
		}
	}
	return ExitFailure
}

func isSilent(err error) bool {
	var ee *ExitError
	return errors.As(err, &ee) && ee.Silent
}
