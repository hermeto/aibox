package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/profile"
)

func newTTLCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ttl",
		Short: "Show or extend the automatic teardown time of the current cloud run",
		Args:  cobra.NoArgs,
	}
	show := &cobra.Command{
		Use:     "show",
		Short:   "Show when the current run is torn down",
		Example: "  aibox ttl show --profile aws-dev",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			p, err := cloudProfile(app)
			if err != nil {
				return err
			}
			a, err := app.awsPlatform(p)
			if err != nil {
				return err
			}
			st, err := a.TTL()
			if err != nil {
				return err
			}
			return app.print(st)
		},
	}
	extend := &cobra.Command{
		Use:     "extend <duration>",
		Short:   "Move the automatic teardown later (at most 24h after the run start)",
		Example: "  aibox ttl extend 2h --profile aws-dev",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := time.ParseDuration(args[0])
			if err != nil {
				return UsageError(fmt.Errorf("invalid duration %q: %w", args[0], err))
			}
			p, err := cloudProfile(app)
			if err != nil {
				return err
			}
			a, err := app.awsPlatform(p)
			if err != nil {
				return err
			}
			st, err := a.ExtendTTL(cmd.Context(), d)
			if err != nil {
				return err
			}
			return app.print(st)
		},
	}
	cmd.AddCommand(show, extend)
	return cmd
}

func cloudProfile(app *App) (*profile.Profile, error) {
	p, err := profile.Load(app.Opts.Profile)
	if err != nil {
		return nil, err
	}
	if !p.IsCloud() {
		return nil, errors.New("profile " + p.Metadata.Name + " is local: TTL and cost apply to cloud profiles only")
	}
	return p, nil
}
