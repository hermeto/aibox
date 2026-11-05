package observability

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hermeto/aibox/internal/cost"
	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/profile"
)

const root = "../.."

func loadProfile(t *testing.T, name string) *profile.Profile {
	t.Helper()
	t.Setenv(profile.EnvProfilesDir, filepath.Join(root, "profiles"))
	p, err := profile.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func catalog(t *testing.T) *models.Catalog {
	t.Helper()
	c, err := models.Load(filepath.Join(root, "models", "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mapping(t *testing.T) *Mapping {
	t.Helper()
	m, err := LoadMapping(root)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// records indexes a group's recording rules by name (the first one for repeated names).
func records(g Group) map[string]Rule {
	out := map[string]Rule{}
	for _, r := range g.Rules {
		if _, ok := out[r.Record]; r.Record != "" && !ok {
			out[r.Record] = r
		}
	}
	return out
}

func alerts(g Group) map[string][]Rule {
	out := map[string][]Rule{}
	for _, r := range g.Rules {
		if r.Alert != "" {
			out[r.Alert] = append(out[r.Alert], r)
		}
	}
	return out
}

func TestRuntimeGroupCoversEveryCanonicalMetric(t *testing.T) {
	m := mapping(t)
	recs := records(RuntimeGroup(m))
	for _, e := range m.Metrics {
		var want []string
		switch e.Type {
		case "gauge":
			want = []string{"aibox:" + e.Name}
		case "counter":
			want = []string{"aibox:" + e.Name + ":rate1m"}
		case "histogram":
			want = []string{"aibox:" + e.Name + "_bucket:rate1m"}
			for _, q := range Quantiles {
				want = append(want, "aibox:"+e.Name+":p"+q)
			}
		default:
			t.Fatalf("%s: unknown type %s", e.Name, e.Type)
		}
		for _, w := range want {
			if _, ok := recs[w]; !ok {
				t.Errorf("%s: no recording rule %s", e.Name, w)
			}
		}
		// The first record of each metric reads the runtime series of both runtimes.
		first := recs[want[0]].Expr
		for _, n := range []string{e.VLLM, e.SGLang} {
			if !strings.Contains(first, n) {
				t.Errorf("%s: %s does not read %s: %s", e.Name, want[0], n, first)
			}
		}
		if !strings.Contains(first, `"model", "$1", "model_name"`) {
			t.Errorf("%s: %s does not relabel model_name to model: %s", e.Name, want[0], first)
		}
	}
	if expr := recs["aibox:kv_cache_usage_ratio"].Expr; !strings.HasPrefix(expr, "avg by (namespace, model)") {
		t.Errorf("ratios average over replicas: %s", expr)
	}
	if expr := recs["aibox:requests_running"].Expr; !strings.HasPrefix(expr, "sum by (namespace, model)") {
		t.Errorf("counts sum over replicas: %s", expr)
	}
	if expr := recs["aibox:time_to_first_token_seconds:p99"].Expr; expr != "histogram_quantile(0.99, sum by (namespace, model, le) (aibox:time_to_first_token_seconds_bucket:rate1m))" {
		t.Errorf("p99: %s", expr)
	}
}

func TestCostGroupPricesEveryInstanceAndCapacity(t *testing.T) {
	table, err := cost.LoadTable(root, "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	g := CostGroup(table)
	prices := map[string]string{}
	for _, r := range g.Rules {
		if r.Record == "aibox:instance_price_usd_per_hour" {
			prices[r.Labels["instance_type"]+"/"+r.Labels["capacity_type"]] = r.Expr
		}
	}
	if len(prices) != len(table.Instances)*len(Capacities) {
		t.Fatalf("%d price series for %d instance types", len(prices), len(table.Instances))
	}
	for key, want := range map[string]string{
		"p5.48xlarge/capacity-block": "vector(31.464)",
		"p5.48xlarge/spot":           "vector(22)",
		"g6.xlarge/on-demand":        "vector(0.8048)",
		"g6.xlarge/spot":             "vector(0.32)",
		"g6.xlarge/capacity-block":   "vector(0.8048)", // no capacity-block price: on-demand
	} {
		if prices[key] != want {
			t.Errorf("%s: %q, want %q", key, prices[key], want)
		}
	}
	recs := records(g)
	if expr := recs["aibox:cluster_cost_usd_per_hour"].Expr; !strings.HasSuffix(expr, "+ 0.145") {
		t.Errorf("cluster cost must add the fixed items (EKS control plane, NAT gateway): %s", expr)
	}
	for _, name := range []string{"aibox:node_price_usd_per_hour", "aibox:pool_cost_usd_per_hour", "aibox:pod_gpu_share",
		"aibox:model_cost_usd_per_hour", "aibox:usd_per_million_output_tokens", "aibox:cluster_usd_per_million_output_tokens"} {
		if _, ok := recs[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	if expr := recs["aibox:usd_per_million_output_tokens"].Expr; !strings.Contains(expr, "> 0") {
		t.Errorf("no division by an idle model's zero token rate: %s", expr)
	}
}

func TestAlertGroup(t *testing.T) {
	served := models.Model{ID: "served", SLO: models.SLO{TTFTP99Ms: 1500}}
	off := false
	unserved := models.Model{ID: "unserved", Serve: &off, SLO: models.SLO{TTFTP99Ms: 100}}
	g := AlertGroup([]models.Model{served, unserved}, false)
	as := alerts(g)
	slo := as["AIBoxTTFTSLOViolation"]
	if len(slo) != 1 {
		t.Fatalf("one SLO alert per served model, got %d", len(slo))
	}
	if slo[0].Expr != `aibox:time_to_first_token_seconds:p99{model="served"} > 1.5` || slo[0].For != SLOFor || slo[0].Labels["model"] != "served" {
		t.Errorf("SLO alert: %+v", slo[0])
	}
	for _, name := range []string{"AIBoxKVCacheSaturated", "AIBoxPodsPending", "AIBoxGPUNodeIdle"} {
		if len(as[name]) != 1 {
			t.Errorf("missing %s", name)
		}
	}
	if len(as["AIBoxPriceMissing"]) != 0 {
		t.Error("price alert without a price table")
	}
	if len(alerts(AlertGroup(nil, true))["AIBoxPriceMissing"]) != 1 {
		t.Error("cloud profiles warn about missing prices")
	}
	for _, r := range g.Rules {
		if r.Labels["severity"] == "" || r.Annotations["summary"] == "" || r.Annotations["description"] == "" || r.For == "" {
			t.Errorf("%s needs severity, summary, description and for", r.Alert)
		}
	}
}

func TestGroupsPerProfile(t *testing.T) {
	c := catalog(t)
	local, err := Build(root, loadProfile(t, "local"), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(local) != 2 || local[0].Name != "aibox-runtime" || local[1].Name != "aibox-alerts" {
		t.Fatalf("local: runtime and alert groups only, got %d groups", len(local))
	}
	slo := alerts(local[1])["AIBoxTTFTSLOViolation"]
	if len(slo) != 1 || slo[0].Labels["model"] != "sim-llama-8b" {
		t.Fatalf("local serves only the simulator: %+v", slo)
	}
	bench, err := Build(root, loadProfile(t, "aws-bench"), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(bench) != 3 || bench[1].Name != "aibox-cost" {
		t.Fatalf("aws-bench: runtime, cost and alert groups, got %d groups", len(bench))
	}
	for _, r := range alerts(bench[2])["AIBoxTTFTSLOViolation"] {
		m := c.Get(r.Labels["model"])
		if m == nil || !m.Served() || !m.For("aws-bench") {
			t.Errorf("SLO alert for %s, which aws-bench does not serve", r.Labels["model"])
		}
	}
	if _, err := Render(loadProfile(t, "local"), local, "json"); err == nil {
		t.Error("unknown format accepted")
	}
}

// TestCommittedRulesMatch fails when the generated rule files drift from the runtime metric
// mapping, the price table, the catalog or the profiles. Fix with `make generate`.
func TestCommittedRulesMatch(t *testing.T) {
	c := catalog(t)
	for _, name := range []string{"local", "aws-dev", "aws-bench"} {
		p := loadProfile(t, name)
		groups, err := Build(root, p, c)
		if err != nil {
			t.Fatal(err)
		}
		for format, path := range map[string]string{
			FormatK8s:        filepath.Join("platform", "observability", "overlays", name, "generated-rules.yaml"),
			FormatPrometheus: filepath.Join("platform", "observability", "rules", name+".rules.yaml"),
		} {
			want, err := Render(p, groups, format)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, path)) //nolint:gosec // repository file
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s is stale: run `make generate`", path)
			}
		}
	}
}
