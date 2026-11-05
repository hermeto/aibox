package exec

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Response is a scripted reply for Fake.
type Response struct {
	Result Result
	Err    error
}

// Fake is a scripted Runner for tests. Responses are keyed by the full command line
// (Cmd.String()) or, as a fallback, by a prefix registered with OnPrefix.
type Fake struct {
	mu       sync.Mutex
	exact    map[string][]Response
	prefixes []prefixResponse
	paths    map[string]string
	Calls    []Cmd
}

type prefixResponse struct {
	prefix string
	resp   Response
}

// NewFake returns an empty Fake. Unscripted commands fail with an error.
func NewFake() *Fake {
	return &Fake{exact: map[string][]Response{}, paths: map[string]string{}}
}

// On scripts the response for an exact command line. Repeated registrations are
// returned in order; the last one repeats.
func (f *Fake) On(cmdline string, res Result, err error) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exact[cmdline] = append(f.exact[cmdline], Response{Result: res, Err: err})
	return f
}

// OnPrefix scripts the response for any command line starting with prefix.
func (f *Fake) OnPrefix(prefix string, res Result, err error) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prefixes = append(f.prefixes, prefixResponse{prefix: prefix, resp: Response{Result: res, Err: err}})
	return f
}

// WithPath makes LookPath(name) succeed.
func (f *Fake) WithPath(name string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths[name] = "/fake/bin/" + name
	return f
}

// Run returns the scripted response and records the call.
func (f *Fake) Run(_ context.Context, cmd Cmd) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, cmd)
	line := cmd.String()
	if rs, ok := f.exact[line]; ok && len(rs) > 0 {
		r := rs[0]
		if len(rs) > 1 {
			f.exact[line] = rs[1:]
		}
		return r.Result, r.Err
	}
	for _, p := range f.prefixes {
		if strings.HasPrefix(line, p.prefix) {
			return p.resp.Result, p.resp.Err
		}
	}
	return Result{ExitCode: 127}, fmt.Errorf("fake: unscripted command %q", line)
}

// LookPath succeeds only for names registered with WithPath.
func (f *Fake) LookPath(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.paths[name]; ok {
		return p, nil
	}
	return "", fmt.Errorf("fake: %s not found in PATH", name)
}

// CommandLines returns the recorded command lines.
func (f *Fake) CommandLines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.Calls))
	for _, c := range f.Calls {
		out = append(out, c.String())
	}
	return out
}
