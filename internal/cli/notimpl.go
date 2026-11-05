package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type stub struct {
	use, short, feature string
}

// notImplemented lists commands that later features deliver.
var notImplemented = []stub{
	{"up", "Bring up the platform for a profile (cluster, GitOps, workloads)", "003 (local) and 004 (aws)"},
	{"down", "Tear down the platform for a profile and check for orphans", "003 (local) and 004 (aws)"},
	{"models", "List and mirror catalog models", "007"},
	{"smoke", "Send one request per model and check metrics, traces and dashboards", "003"},
	{"bench", "Run a declared benchmark experiment", "013"},
	{"report", "Generate the benchmark report from results", "014"},
	{"cost", "Show the accumulated cost of the current cloud run", "005"},
}

func newNotImplementedCmd(s stub) *cobra.Command {
	return &cobra.Command{
		Use:     s.use,
		Short:   s.short,
		Example: fmt.Sprintf("  aibox %s --profile local", s.use),
		Args:    cobra.ArbitraryArgs,
		RunE: func(*cobra.Command, []string) error {
			return &ExitError{Code: ExitNotImplemented, Err: fmt.Errorf("aibox %s: not implemented yet (planned in feature %s) — see %s", s.use, s.feature, RoadmapURL)}
		},
	}
}
