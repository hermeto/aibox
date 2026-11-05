package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/awsapi"
	"github.com/hermeto/aibox/internal/doctor"
	"github.com/hermeto/aibox/internal/profile"
)

func newDoctorCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check prerequisites for a profile (tools, container runtime, AWS quotas)",
		Long: `doctor runs preflight checks for the selected profile and reports pass, warn, fail or
skip per check with a remediation hint. AWS checks are read-only and are skipped when no
credentials are configured. The exit code is 1 if any check fails.`,
		Example: "  aibox doctor --profile local\n  aibox doctor --profile aws-dev --output json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			factory := app.AWS
			if factory == nil {
				factory = awsapi.New
			}
			report := doctor.Run(cmd.Context(), p, doctor.Env{
				Runner: app.readOnlyRunner(),
				AWS:    factory,
				Getenv: app.getenv,
				GOOS:   goos(),
			})
			if err := app.print(report); err != nil {
				return err
			}
			if !report.OK {
				return &ExitError{Code: ExitFailure, Err: errors.New("doctor: one or more checks failed"), Silent: true}
			}
			return nil
		},
	}
}
