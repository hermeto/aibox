package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/platform"
	"github.com/hermeto/aibox/internal/profile"
	"github.com/hermeto/aibox/internal/smoke"
)

// localModels are the models the local profile serves (feature 007 moves this to the catalog).
var localModels = []string{"sim-llama-8b"}

type downResult struct {
	Profile string `json:"profile"`
	Cluster string `json:"cluster"`
	Deleted bool   `json:"deleted"`
}

func (d downResult) RenderText(w io.Writer) error {
	msg := "deleted"
	if !d.Deleted {
		msg = "did not exist (nothing to delete)"
	}
	_, err := fmt.Fprintf(w, "Cluster %s (profile %s) %s.\n", d.Cluster, d.Profile, msg)
	return err
}

func newSmokeCmd(app *App) *cobra.Command {
	var o smoke.Options
	cmd := &cobra.Command{
		Use:   "smoke",
		Short: "Send one request per model and check metrics, traces and dashboards",
		Long: `smoke lists the models on the inference endpoint, sends one chat completion per model
and checks that the runtime metrics AIBox depends on are exposed. Exit code 1 on any failure.`,
		Example: "  aibox smoke --profile local\n  aibox smoke --profile local --output json | jq '.checks[] | select(.status != \"pass\")'",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			if p.IsCloud() {
				return notImplementedFor("smoke", p.Metadata.Name, "009")
			}
			o.Profile = p.Metadata.Name
			if o.BaseURL == "" {
				o.BaseURL = platform.LocalEndpoints[0].URL
			}
			if o.MetricsURL == "" {
				o.MetricsURL = strings.TrimSuffix(o.BaseURL, "/v1") + "/metrics"
			}
			if len(o.ExpectModels) == 0 {
				o.ExpectModels = localModels
			}
			report := smoke.Run(cmd.Context(), o)
			if err := app.print(report); err != nil {
				return err
			}
			if !report.OK {
				return &ExitError{Code: ExitFailure, Err: errors.New("smoke: one or more checks failed"), Silent: true}
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.BaseURL, "endpoint", "", "OpenAI-compatible base URL (default: the profile's inference endpoint)")
	f.StringVar(&o.MetricsURL, "metrics-url", "", "Prometheus metrics URL to check (default: <endpoint without /v1>/metrics)")
	f.StringSliceVar(&o.Models, "model", nil, "model(s) to test (default: every listed model)")
	f.StringSliceVar(&o.ExpectModels, "expect-model", nil, "model(s) that must be listed (default: the profile's models)")
	return cmd
}
