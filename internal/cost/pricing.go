// Package cost implements AIBox's cost guardrails in Go: the versioned price table, the
// run cost report (`aibox cost`), the pre-run estimate and the read-only orphan checker.
package cost

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"sigs.k8s.io/yaml"
)

// Price is the hourly price of one instance type.
type Price struct {
	OnDemand      float64 `json:"onDemand"`
	Spot          float64 `json:"spot"`
	CapacityBlock float64 `json:"capacityBlock,omitempty"`
}

// Table is a versioned price table (data/pricing/aws-<region>.yaml).
type Table struct {
	Region      string             `json:"region"`
	Currency    string             `json:"currency"`
	Source      string             `json:"source"`
	LastUpdated string             `json:"lastUpdated"`
	Verified    bool               `json:"verified"`
	Instances   map[string]Price   `json:"instances"`
	Fixed       map[string]float64 `json:"fixed"`
}

// MaxAge after which a price table is considered stale.
const MaxAge = 90 * 24 * time.Hour

// LoadTable reads data/pricing/aws-<region>.yaml under root.
func LoadTable(root, region string) (*Table, error) {
	path := filepath.Join(root, "data", "pricing", "aws-"+region+".yaml")
	data, err := os.ReadFile(path) //nolint:gosec // repository file
	if err != nil {
		return nil, fmt.Errorf("price table for %s: %w", region, err)
	}
	var t Table
	if err := yaml.UnmarshalStrict(data, &t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := t.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &t, nil
}

// Validate checks required fields.
func (t *Table) Validate() error {
	var errs []error
	if t.Region == "" || t.Currency == "" || t.Source == "" {
		errs = append(errs, errors.New("region, currency and source are required"))
	}
	if _, err := time.Parse("2006-01-02", t.LastUpdated); err != nil {
		errs = append(errs, fmt.Errorf("lastUpdated must be YYYY-MM-DD: %w", err))
	}
	names := make([]string, 0, len(t.Instances))
	for n := range t.Instances {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if p := t.Instances[n]; p.OnDemand <= 0 || p.Spot < 0 {
			errs = append(errs, fmt.Errorf("instance %s: onDemand must be > 0 and spot >= 0", n))
		}
	}
	return errors.Join(errs...)
}

// Age returns how old the table is at now.
func (t *Table) Age(now time.Time) time.Duration {
	d, err := time.Parse("2006-01-02", t.LastUpdated)
	if err != nil {
		return MaxAge * 10
	}
	return now.Sub(d)
}

// Hourly returns the hourly price of an instance for a capacity type
// ("spot", "on-demand" or "capacity-block").
func (t *Table) Hourly(instanceType, capacity string) (float64, bool) {
	p, ok := t.Instances[instanceType]
	if !ok {
		return 0, false
	}
	switch capacity {
	case "spot":
		if p.Spot > 0 {
			return p.Spot, true
		}
		return p.OnDemand, true
	case "capacity-block":
		if p.CapacityBlock > 0 {
			return p.CapacityBlock, true
		}
		return p.OnDemand, true
	default:
		return p.OnDemand, true
	}
}
