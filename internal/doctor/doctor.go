// Package doctor implements `aibox doctor`: preflight checks for a profile. Local checks
// cover tools and the container runtime; AWS checks are strictly read-only.
package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hermeto/aibox/internal/cost"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
)

// Status of a check.
type Status string

// Check statuses.
const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
	Skip Status = "skip"
)

// Check is the result of one preflight check.
type Check struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      Status            `json:"status"`
	Message     string            `json:"message"`
	Remediation string            `json:"remediation,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

// Summary counts checks by status.
type Summary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// Report is the outcome of a doctor run.
type Report struct {
	Profile  string  `json:"profile"`
	Provider string  `json:"provider"`
	Checks   []Check `json:"checks"`
	Summary  Summary `json:"summary"`
	OK       bool    `json:"ok"`
}

// Env holds the dependencies of a doctor run (injected for tests).
type Env struct {
	Runner exec.Runner
	// AWS builds a read-only AWS client for a region. Nil disables AWS checks.
	AWS AWSFactory
	// Getenv reads environment variables (defaults to os.Getenv).
	Getenv func(string) string
	// GOOS selects install hints (defaults to runtime.GOOS via the caller).
	GOOS string
	// PriceTable enables the cost estimate check for cloud profiles.
	PriceTable *cost.Table
	// BudgetUSD is the run budget, compared with the estimate (0 = not set).
	BudgetUSD float64
	// TTL of the planned run (0 = profile default).
	TTL time.Duration
	// Now is the clock (nil = time.Now).
	Now func() time.Time
}

func (e Env) getenv(k string) string {
	if e.Getenv != nil {
		return e.Getenv(k)
	}
	return os.Getenv(k)
}

// Run executes every check that applies to the profile.
func Run(ctx context.Context, p *profile.Profile, env Env) *Report {
	r := &Report{Profile: p.Metadata.Name, Provider: string(p.Spec.Provider)}
	switch p.Spec.Provider {
	case profile.ProviderKind:
		r.add(toolChecks(ctx, env, kindTools)...)
		r.add(dockerChecks(ctx, env, p.Spec.Local)...)
	case profile.ProviderAWS:
		r.add(toolChecks(ctx, env, awsTools)...)
		r.add(awsChecks(ctx, env, p.Spec.AWS)...)
		if env.PriceTable != nil {
			r.add(estimateCheck(p, env))
		}
	}
	return r.finish()
}

func (r *Report) add(cs ...Check) { r.Checks = append(r.Checks, cs...) }

func (r *Report) finish() *Report {
	r.Summary = Summary{}
	for _, c := range r.Checks {
		switch c.Status {
		case Pass:
			r.Summary.Pass++
		case Warn:
			r.Summary.Warn++
		case Fail:
			r.Summary.Fail++
		case Skip:
			r.Summary.Skip++
		}
	}
	r.OK = r.Summary.Fail == 0
	return r
}

// RenderText prints a table of checks followed by remediation hints.
func (r *Report) RenderText(w io.Writer) error {
	ew := &errWriter{w: w}
	tw := tabwriter.NewWriter(ew, 0, 2, 2, ' ', 0)
	ew.printf("aibox doctor — profile %s (%s)\n\n", r.Profile, r.Provider)
	fmt.Fprintln(tw, "STATUS\tCHECK\tDETAILS") //nolint:errcheck // tabwriter errors surface on Flush
	for _, c := range r.Checks {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", strings.ToUpper(string(c.Status)), c.Name, c.Message) //nolint:errcheck // see above
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	var hints []string
	for _, c := range r.Checks {
		if c.Remediation != "" && (c.Status == Fail || c.Status == Warn) {
			hints = append(hints, fmt.Sprintf("  - %s: %s", c.Name, c.Remediation))
		}
	}
	if len(hints) > 0 {
		ew.printf("\nRemediation:\n%s\n", strings.Join(hints, "\n"))
	}
	verdict := "OK"
	if !r.OK {
		verdict = "FAILED"
	}
	ew.printf("\n%s: %d pass, %d warn, %d fail, %d skip\n", verdict, r.Summary.Pass, r.Summary.Warn, r.Summary.Fail, r.Summary.Skip)
	return ew.err
}

// errWriter remembers the first write error so rendering code stays linear.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	n, err := e.w.Write(p)
	e.err = err
	return n, err
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err == nil {
		_, e.err = fmt.Fprintf(e.w, format, args...)
	}
}

func estimateCheck(p *profile.Profile, env Env) Check {
	now := time.Now()
	if env.Now != nil {
		now = env.Now()
	}
	ttl := env.TTL
	if ttl == 0 {
		ttl, _ = p.Spec.Guardrails.TTLDuration()
	}
	e := cost.EstimateRun(p, env.PriceTable, ttl, now)
	c := Check{ID: "cost.estimate", Name: "cost estimate", Status: Pass, Message: e.Summary(), Details: map[string]string{
		"worstCaseUSD": fmt.Sprintf("%.2f", e.Total), "perHourUSD": fmt.Sprintf("%.2f", e.PerHour), "priceTableDate": e.TableDate,
	}}
	var issues []string
	if env.BudgetUSD > 0 && env.BudgetUSD < e.Total {
		issues = append(issues, fmt.Sprintf("budget USD %.2f is below the worst case USD %.2f", env.BudgetUSD, e.Total))
	}
	if env.BudgetUSD == 0 {
		c.Remediation = fmt.Sprintf("set BUDGET_USD (>= %.0f recommended) before aibox up", e.Total)
	}
	if e.StaleTable {
		issues = append(issues, "price table older than 90 days")
	}
	if !e.Verified {
		issues = append(issues, "price table not verified against the AWS Price List API")
	}
	if len(issues) > 0 {
		c.Status = Warn
		c.Message += "; " + strings.Join(issues, "; ")
		c.Remediation = "refresh prices with `go run ./hack/pricing` and/or raise the budget"
	}
	return c
}
