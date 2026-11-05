package cli

import (
	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/gpu"
	"github.com/hermeto/aibox/internal/profile"
)

func newRenderCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render generated platform manifests from a profile",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:     "nodepools",
		Short:   "Render Karpenter EC2NodeClasses and NodePools for the profile's GPU classes",
		Example: "  aibox render nodepools --profile aws-bench > platform/gpu/overlays/aws-bench/nodepools.yaml",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			out, err := gpu.Render(p)
			if err != nil {
				return err
			}
			_, err = app.Out.Write(out)
			return err
		},
	})
	return cmd
}
