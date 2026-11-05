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
	{"bench", "Run a declared benchmark experiment", "013"},
	{"report", "Generate the benchmark report from results", "014"},
}

// notImplementedFor returns the exit-2 error for a command that exists for some profiles only.
func notImplementedFor(cmd, profileName, feature string) error {
	return &ExitError{Code: ExitNotImplemented, Err: fmt.Errorf("aibox %s --profile %s: not implemented yet (planned in feature %s) — see %s", cmd, profileName, feature, RoadmapURL)}
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
