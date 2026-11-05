package cli

import (
	"github.com/spf13/cobra"
)

func newCostCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "cost",
		Short: "Show the accumulated cost of the current cloud run",
		Long: `cost prices the current run: EC2 instance-hours of instances tagged with the run ID ×
the versioned price table (data/pricing), plus fixed hourly items such as the EKS control
plane and the NAT gateway. It is an estimate; AWS Cost Explorer data lags by up to 24 hours.`,
		Example: "  aibox cost --profile aws-dev\n  aibox cost --profile aws-dev -o json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := cloudProfile(app)
			if err != nil {
				return err
			}
			a, err := app.awsPlatform(p)
			if err != nil {
				return err
			}
			e, _, err := app.costClients()(cmd.Context(), p.Spec.AWS.Region)
			if err != nil {
				return err
			}
			report, err := a.Cost(cmd.Context(), e)
			if err != nil {
				return err
			}
			return app.print(report)
		},
	}
}
