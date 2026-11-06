package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// dashboard is the subset of the Grafana dashboard model AIBox relies on. The dashboards are
// committed as JSON and loaded by the Grafana sidecar, so CI lints them without a running
// cluster (the plan's dashboard-linter step is implemented here as a Go test: the upstream
// linter cannot be `go install`ed because its module contains replace directives).
type dashboard struct {
	UID        string `json:"uid"`
	Title      string `json:"title"`
	Panels     []panel
	Tags       []string
	Templating struct {
		List []any `json:"list"`
	} `json:"templating"`
}

type panel struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Targets []struct {
		Expr       string `json:"expr"`
		LegendFmt  string `json:"legendFormat"`
		RefID      string `json:"refId"`
		Datasource any    `json:"datasource"`
	} `json:"targets"`
	GridPos struct {
		H int `json:"h"`
		W int `json:"w"`
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"gridPos"`
}

func TestDashboards(t *testing.T) {
	dir := filepath.Join("..", "..", "platform", "observability", "base", "dashboards")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	want := map[string]bool{
		"aibox-slo": true, "aibox-capacity": true, "aibox-gpu": true,
		"aibox-cost": true, "aibox-scaling": true,
	}
	seenUID := map[string]string{}
	seenID := map[string]bool{}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		var d dashboard
		if err := json.Unmarshal(data, &d); err != nil {
			t.Errorf("%s: invalid JSON: %v", name, err)
			continue
		}
		if d.UID == "" || d.Title == "" {
			t.Errorf("%s: uid and title are required", name)
		}
		if !want[d.UID] {
			t.Errorf("%s: unexpected uid %q (want one of the five product dashboards)", name, d.UID)
		}
		if prev, ok := seenUID[d.UID]; ok {
			t.Errorf("%s: duplicate uid %q (also in %s)", name, d.UID, prev)
		}
		seenUID[d.UID] = name
		if len(d.Panels) == 0 {
			t.Errorf("%s: no panels", name)
		}
		for _, p := range d.Panels {
			key := fmt.Sprintf("%s/%d", d.UID, p.ID)
			if p.ID <= 0 {
				t.Errorf("%s: panel with non-positive id", name)
			}
			if seenID[key] {
				t.Errorf("%s: duplicate panel id %d", name, p.ID)
			}
			seenID[key] = true
			if p.Title == "" {
				t.Errorf("%s: panel %d has no title", name, p.ID)
			}
			switch p.Type {
			case "timeseries", "stat", "gauge", "table", "bargauge", "heatmap":
			default:
				t.Errorf("%s: panel %d has unsupported type %q", name, p.ID, p.Type)
			}
			if p.GridPos.W <= 0 || p.GridPos.H <= 0 || p.GridPos.X < 0 || p.GridPos.X+p.GridPos.W > 24 {
				t.Errorf("%s: panel %d has an invalid grid position %+v", name, p.ID, p.GridPos)
			}
			if len(p.Targets) == 0 {
				t.Errorf("%s: panel %d has no targets", name, p.ID)
			}
			for _, target := range p.Targets {
				if strings.TrimSpace(target.Expr) == "" {
					t.Errorf("%s: panel %d has an empty query", name, p.ID)
				}
				if target.RefID == "" {
					t.Errorf("%s: panel %d has a query without refId", name, p.ID)
				}
			}
		}
	}
	if len(files) != len(want) {
		t.Errorf("found %d dashboards, want %d", len(files), len(want))
	}
	for uid := range want {
		if _, ok := seenUID[uid]; !ok {
			t.Errorf("missing dashboard %q", uid)
		}
	}
}
