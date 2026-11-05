// Package smoke implements `aibox smoke`: one OpenAI-compatible request per model plus
// checks that the runtime metrics AIBox depends on are exposed.
package smoke

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// RequiredSeries are the runtime metric names later features depend on (routing,
// autoscaling, dashboards). Histograms are matched by their _bucket series.
var RequiredSeries = []string{
	"vllm:num_requests_running",
	"vllm:num_requests_waiting",
	"vllm:kv_cache_usage_perc",
	"vllm:time_to_first_token_seconds_bucket",
}

// Status of a smoke check.
type Status string

// Check statuses.
const (
	Pass Status = "pass"
	Fail Status = "fail"
)

// Check is one smoke check result.
type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message"`
}

// Report is the outcome of a smoke run.
type Report struct {
	Profile  string  `json:"profile"`
	Endpoint string  `json:"endpoint"`
	Checks   []Check `json:"checks"`
	OK       bool    `json:"ok"`
}

// Options configure a smoke run.
type Options struct {
	Profile string
	// BaseURL is the OpenAI-compatible base, e.g. http://127.0.0.1:30080/v1.
	BaseURL string
	// MetricsURL is scraped for RequiredSeries; empty skips the metrics check.
	MetricsURL string
	// Models restricts the run; empty means every model listed by /models.
	Models []string
	// ExpectModels must appear in /models.
	ExpectModels []string
	// Header is added to every request (e.g. Authorization from feature 009).
	Header http.Header
	Client *http.Client
}

// Run executes the smoke checks.
func Run(ctx context.Context, o Options) *Report {
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 30 * time.Second}
	}
	r := &Report{Profile: o.Profile, Endpoint: o.BaseURL}
	listed, check := listModels(ctx, o)
	r.Checks = append(r.Checks, check)
	models := o.Models
	if len(models) == 0 {
		models = listed
	}
	for _, m := range models {
		r.Checks = append(r.Checks, chatCompletion(ctx, o, m))
	}
	if o.MetricsURL != "" {
		r.Checks = append(r.Checks, metrics(ctx, o))
	}
	r.OK = true
	for _, c := range r.Checks {
		if c.Status != Pass {
			r.OK = false
		}
	}
	return r
}

// RenderText prints one line per check.
func (r *Report) RenderText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "aibox smoke — profile %s (%s)\n\n", r.Profile, r.Endpoint)
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "%-5s %-40s %s\n", strings.ToUpper(string(c.Status)), c.Name, c.Message)
	}
	verdict := "OK"
	if !r.OK {
		verdict = "FAILED"
	}
	fmt.Fprintf(&b, "\n%s\n", verdict)
	_, err := io.WriteString(w, b.String())
	return err
}

func (o Options) do(ctx context.Context, method, url string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return 0, nil, err
	}
	for k, vs := range o.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, data, err
}

func listModels(ctx context.Context, o Options) ([]string, Check) {
	c := Check{Name: "models/list"}
	status, data, err := o.do(ctx, http.MethodGet, strings.TrimRight(o.BaseURL, "/")+"/models", nil)
	if err != nil {
		c.Status, c.Message = Fail, err.Error()
		return nil, c
	}
	if status != http.StatusOK {
		c.Status, c.Message = Fail, fmt.Sprintf("HTTP %d: %s", status, truncate(data))
		return nil, c
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		c.Status, c.Message = Fail, "invalid JSON: "+err.Error()
		return nil, c
	}
	var ids []string
	for _, d := range body.Data {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	for _, want := range o.ExpectModels {
		if !contains(ids, want) {
			c.Status, c.Message = Fail, fmt.Sprintf("%s not listed (got %s)", want, strings.Join(ids, ", "))
			return ids, c
		}
	}
	if len(ids) == 0 {
		c.Status, c.Message = Fail, "no models listed"
		return ids, c
	}
	c.Status, c.Message = Pass, strings.Join(ids, ", ")+" listed"
	return ids, c
}

func chatCompletion(ctx context.Context, o Options, model string) Check {
	c := Check{Name: model + "/chat-completion"}
	req := map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "Say hello in five words."}},
		"max_tokens": 16,
	}
	start := time.Now()
	status, data, err := o.do(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+"/chat/completions", req)
	if err != nil {
		c.Status, c.Message = Fail, err.Error()
		return c
	}
	if status != http.StatusOK {
		c.Status, c.Message = Fail, fmt.Sprintf("HTTP %d: %s", status, truncate(data))
		return c
	}
	var body struct {
		Choices []json.RawMessage `json:"choices"`
		Usage   struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &body); err != nil || len(body.Choices) == 0 {
		c.Status, c.Message = Fail, "response has no choices: "+truncate(data)
		return c
	}
	c.Status = Pass
	c.Message = fmt.Sprintf("HTTP 200, %d choice(s), %d completion tokens, %s", len(body.Choices), body.Usage.CompletionTokens, time.Since(start).Round(time.Millisecond))
	return c
}

func metrics(ctx context.Context, o Options) Check {
	c := Check{Name: "metrics/required-series"}
	status, data, err := o.do(ctx, http.MethodGet, o.MetricsURL, nil)
	if err != nil {
		c.Status, c.Message = Fail, err.Error()
		return c
	}
	if status != http.StatusOK {
		c.Status, c.Message = Fail, fmt.Sprintf("HTTP %d", status)
		return c
	}
	found := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		for _, s := range RequiredSeries {
			if strings.HasPrefix(line, s+"{") || strings.HasPrefix(line, s+" ") {
				found[s] = true
			}
		}
	}
	var missing []string
	for _, s := range RequiredSeries {
		if !found[s] {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		c.Status, c.Message = Fail, "missing: "+strings.Join(missing, ", ")
		return c
	}
	c.Status, c.Message = Pass, fmt.Sprintf("%d/%d series present", len(RequiredSeries), len(RequiredSeries))
	return c
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func truncate(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
