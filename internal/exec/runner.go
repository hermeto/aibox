// Package exec runs external commands (kind, helm, tofu, docker, ...) behind a small
// interface so that every caller gets dry-run support and recorded fakes for tests.
package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	osexec "os/exec"
	"strings"
)

// Cmd describes one external command invocation.
type Cmd struct {
	Name  string
	Args  []string
	Dir   string
	Env   []string  // extra KEY=VALUE pairs appended to the current environment
	Stdin io.Reader // optional
	// Stream, when set, receives stdout and stderr as they are produced (in addition to
	// being captured in Result).
	Stream io.Writer
}

// String renders the command line for logs and dry-run output.
func (c Cmd) String() string {
	parts := append([]string{c.Name}, c.Args...)
	for i, p := range parts {
		if p == "" || strings.ContainsAny(p, " \t\"'$") {
			parts[i] = fmt.Sprintf("%q", p)
		}
	}
	return strings.Join(parts, " ")
}

// Result is the outcome of a command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes commands.
type Runner interface {
	Run(ctx context.Context, cmd Cmd) (Result, error)
	// LookPath reports the absolute path of an executable, like exec.LookPath.
	LookPath(name string) (string, error)
}

// ErrExit is returned (wrapped) when a command exits with a non-zero status.
var ErrExit = errors.New("command exited with non-zero status")

// Real runs commands with os/exec.
type Real struct {
	Logger *slog.Logger
}

// Run executes cmd and captures its output.
func (r Real) Run(ctx context.Context, cmd Cmd) (Result, error) {
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Debug("exec", "cmd", cmd.String(), "dir", cmd.Dir)
	c := osexec.CommandContext(ctx, cmd.Name, cmd.Args...) //nolint:gosec // callers pass fixed tool names
	c.Dir = cmd.Dir
	if len(cmd.Env) > 0 {
		c.Env = append(c.Environ(), cmd.Env...)
	}
	c.Stdin = cmd.Stdin
	var stdout, stderr bytes.Buffer
	if cmd.Stream != nil {
		c.Stdout = io.MultiWriter(&stdout, cmd.Stream)
		c.Stderr = io.MultiWriter(&stderr, cmd.Stream)
	} else {
		c.Stdout = &stdout
		c.Stderr = &stderr
	}
	err := c.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		var exitErr *osexec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, fmt.Errorf("%s: %w (exit %d): %s", cmd.Name, ErrExit, res.ExitCode, strings.TrimSpace(res.Stderr))
		}
		res.ExitCode = -1
		return res, fmt.Errorf("%s: %w", cmd.Name, err)
	}
	return res, nil
}

// LookPath wraps exec.LookPath.
func (Real) LookPath(name string) (string, error) { return osexec.LookPath(name) }

// DryRun prints mutating commands instead of running them. Commands marked read-only by
// the ReadOnly predicate are delegated to Inner so that dry runs can still inspect state.
type DryRun struct {
	Inner    Runner
	Out      io.Writer
	ReadOnly func(Cmd) bool
}

// Run prints the command (or delegates read-only commands).
func (d DryRun) Run(ctx context.Context, cmd Cmd) (Result, error) {
	if d.ReadOnly != nil && d.ReadOnly(cmd) && d.Inner != nil {
		return d.Inner.Run(ctx, cmd)
	}
	if d.Out != nil {
		if _, err := fmt.Fprintf(d.Out, "[dry-run] %s\n", cmd.String()); err != nil {
			return Result{}, err
		}
	}
	return Result{}, nil
}

// LookPath delegates to the inner runner when present.
func (d DryRun) LookPath(name string) (string, error) {
	if d.Inner != nil {
		return d.Inner.LookPath(name)
	}
	return "/usr/bin/" + name, nil
}

// ReadOnly reports whether a command only inspects state, so dry runs may execute it:
// git, version queries, `kind get`, `kubectl kustomize`, `helm template`, `docker info`.
func ReadOnly(c Cmd) bool {
	first := ""
	if len(c.Args) > 0 {
		first = c.Args[0]
	}
	switch c.Name {
	case "git":
		return true
	case "kind":
		return first == "get" || first == "version"
	case "kubectl":
		return first == "kustomize" || first == "version"
	case "helm":
		return first == "template" || first == "version"
	case "docker":
		return first == "info" || first == "--version" || first == "version"
	case "tofu", "terraform":
		return first == "version"
	}
	return false
}
