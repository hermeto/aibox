package cost

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hermeto/aibox/internal/profile"
)

// SystemNodes is the system node group used for estimates (infra/terraform/aws defaults).
var SystemNodes = struct {
	Type  string
	Count int
}{Type: "m7i.xlarge", Count: 2}

// Estimate is the worst-case cost of a planned run.
type Estimate struct {
	TTL        time.Duration `json:"ttl"`
	PerHour    float64       `json:"perHour"`
	Total      float64       `json:"total"`
	Breakdown  []string      `json:"breakdown"`
	Unknown    []string      `json:"unknownTypes,omitempty"`
	TableDate  string        `json:"tableDate"`
	Verified   bool          `json:"tableVerified"`
	StaleTable bool          `json:"staleTable"`
}

// EstimateRun returns the worst-case cost: every GPU class at maxNodes of its most expensive
// instance type, the system nodes and fixed items, for the whole TTL.
func EstimateRun(p *profile.Profile, table *Table, ttl time.Duration, now time.Time) *Estimate {
	e := &Estimate{TTL: ttl, TableDate: table.LastUpdated, Verified: table.Verified, StaleTable: table.Age(now) > MaxAge}
	add := func(label string, perHour float64) {
		e.PerHour += perHour
		e.Breakdown = append(e.Breakdown, fmt.Sprintf("%s: USD %.2f/h", label, perHour))
	}
	if sys, ok := table.Hourly(SystemNodes.Type, "on-demand"); ok {
		add(fmt.Sprintf("%d × %s (system)", SystemNodes.Count, SystemNodes.Type), float64(SystemNodes.Count)*sys)
	} else {
		e.Unknown = append(e.Unknown, SystemNodes.Type)
	}
	if p.Spec.AWS != nil {
		for _, c := range p.Spec.AWS.GPUClasses {
			worst, worstType := 0.0, ""
			for _, t := range c.InstanceTypes {
				price, ok := table.Hourly(t, c.CapacityType)
				if !ok {
					e.Unknown = append(e.Unknown, t)
					continue
				}
				if price > worst {
					worst, worstType = price, t
				}
			}
			if worstType != "" && c.MaxNodes > 0 {
				add(fmt.Sprintf("%d × %s (%s, %s)", c.MaxNodes, worstType, c.Name, c.CapacityType), float64(c.MaxNodes)*worst)
			}
		}
	}
	names := make([]string, 0, len(table.Fixed))
	for n := range table.Fixed {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		add(n, table.Fixed[n])
	}
	e.PerHour = round2(e.PerHour)
	e.Total = round2(e.PerHour * ttl.Hours())
	sort.Strings(e.Unknown)
	return e
}

// Summary is a one-line description of the estimate.
func (e *Estimate) Summary() string {
	s := fmt.Sprintf("worst case USD %.2f for %s (USD %.2f/h; prices of %s)", e.Total, e.TTL, e.PerHour, e.TableDate)
	if len(e.Unknown) > 0 {
		s += "; unknown prices: " + strings.Join(e.Unknown, ", ")
	}
	return s
}
