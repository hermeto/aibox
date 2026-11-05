package cost

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// LineItem is one priced component of a run.
type LineItem struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Capacity     string  `json:"capacity,omitempty"`
	Hours        float64 `json:"hours"`
	PricePerHour float64 `json:"pricePerHour"`
	Cost         float64 `json:"cost"`
	PriceKnown   bool    `json:"priceKnown"`
}

// Report is the accumulated cost of a run.
type Report struct {
	RunID      string     `json:"runId"`
	Region     string     `json:"region"`
	From       time.Time  `json:"from"`
	To         time.Time  `json:"to"`
	Items      []LineItem `json:"items"`
	Total      float64    `json:"total"`
	LowerBound bool       `json:"lowerBound"`
	Notes      []string   `json:"notes"`
}

// RenderText prints the line items and the total.
func (r *Report) RenderText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Cost of run %s (%s) since %s\n\n", r.RunID, r.Region, r.From.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "%-34s %-14s %-10s %8s %10s %10s\n", "ITEM", "TYPE", "CAPACITY", "HOURS", "USD/HOUR", "USD")
	for _, it := range r.Items {
		price := fmt.Sprintf("%.4f", it.PricePerHour)
		if !it.PriceKnown {
			price = "unknown"
		}
		fmt.Fprintf(&b, "%-34s %-14s %-10s %8.2f %10s %10.2f\n", it.Name, it.Type, it.Capacity, it.Hours, price, it.Cost)
	}
	qual := ""
	if r.LowerBound {
		qual = " (lower bound: some prices unknown)"
	}
	fmt.Fprintf(&b, "\nTotal: USD %.2f%s\n", r.Total, qual)
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "Note: %s\n", n)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RunCost prices the run's EC2 instances (tagged run-id) plus fixed hourly items since start.
func RunCost(ctx context.Context, ec2c EC2API, table *Table, runID string, start, now time.Time) (*Report, error) {
	r := &Report{RunID: runID, Region: table.Region, From: start, To: now, Items: []LineItem{}}
	var token *string
	for {
		out, err := ec2c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{Filters: tagFilter(runID), NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("describe instances: %w", err)
		}
		for _, res := range out.Reservations {
			for _, in := range res.Instances {
				if in.State == nil || in.State.Name == ec2types.InstanceStateNameTerminated || in.LaunchTime == nil {
					continue
				}
				capacity := "on-demand"
				if in.InstanceLifecycle == ec2types.InstanceLifecycleTypeSpot {
					capacity = "spot"
				}
				if in.CapacityReservationId != nil {
					capacity = "capacity-block"
				}
				hours := hoursBetween(*in.LaunchTime, now)
				price, known := table.Hourly(string(in.InstanceType), capacity)
				r.Items = append(r.Items, LineItem{Name: aws.ToString(in.InstanceId), Type: string(in.InstanceType), Capacity: capacity,
					Hours: hours, PricePerHour: price, Cost: round2(hours * price), PriceKnown: known})
				if !known {
					r.LowerBound = true
				}
			}
		}
		if token = out.NextToken; token == nil {
			break
		}
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].Name < r.Items[j].Name })
	hours := hoursBetween(start, now)
	fixed := make([]string, 0, len(table.Fixed))
	for name := range table.Fixed {
		fixed = append(fixed, name)
	}
	sort.Strings(fixed)
	for _, name := range fixed {
		p := table.Fixed[name]
		r.Items = append(r.Items, LineItem{Name: name, Type: "fixed", Hours: hours, PricePerHour: p, Cost: round2(hours * p), PriceKnown: true})
	}
	for _, it := range r.Items {
		r.Total += it.Cost
	}
	r.Total = round2(r.Total)
	r.Notes = append(r.Notes,
		"estimate from instance-hours × the versioned price table ("+table.LastUpdated+"); AWS Cost Explorer data lags by up to 24 hours",
		"excludes data transfer, NAT data processing, EBS and S3 storage")
	if !table.Verified {
		r.Notes = append(r.Notes, "price table not verified against the AWS Price List API (run go run ./hack/pricing)")
	}
	return r, nil
}

func hoursBetween(from, to time.Time) float64 {
	if to.Before(from) {
		return 0
	}
	return math.Round(to.Sub(from).Hours()*100) / 100
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
