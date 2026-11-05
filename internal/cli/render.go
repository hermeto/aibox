package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/gpu"
	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/observability"
	"github.com/hermeto/aibox/internal/profile"
	"github.com/hermeto/aibox/internal/serving"
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
	var catalogPath string
	servingCmd := &cobra.Command{
		Use:     "serving",
		Short:   "Render the profile's model servers (KServe LLMInferenceServices) from the model catalog",
		Example: "  aibox render serving --profile aws-bench > platform/serving/overlays/aws-bench/models.yaml",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			if catalogPath == "" {
				catalogPath = models.DefaultPath()
			}
			c, err := models.Load(catalogPath)
			if err != nil {
				return err
			}
			out, err := serving.Render(p, c)
			if err != nil {
				return err
			}
			_, err = app.Out.Write(out)
			return err
		},
	}
	servingCmd.Flags().StringVar(&catalogPath, "catalog", "", "catalog file (default: models/catalog.yaml in the repository, or $AIBOX_CATALOG)")
	cmd.AddCommand(servingCmd)

	var format string
	rulesCmd := &cobra.Command{
		Use:     "rules",
		Short:   "Render the Prometheus rules: canonical runtime series, live cost and alerts",
		Example: "  aibox render rules --profile aws-bench\n  aibox render rules --profile local --format prometheus",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if format != observability.FormatK8s && format != observability.FormatPrometheus {
				return UsageError(fmt.Errorf("invalid --format %q (want %s or %s)", format, observability.FormatK8s, observability.FormatPrometheus))
			}
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			root, err := profile.RepoRoot()
			if err != nil {
				return err
			}
			c, err := models.Load(models.DefaultPath())
			if err != nil {
				return err
			}
			groups, err := observability.Build(root, p, c)
			if err != nil {
				return err
			}
			out, err := observability.Render(p, groups, format)
			if err != nil {
				return err
			}
			_, err = app.Out.Write(out)
			return err
		},
	}
	rulesCmd.Flags().StringVar(&format, "format", observability.FormatK8s, "k8s (PrometheusRule) or prometheus (rule file for promtool)")
	cmd.AddCommand(rulesCmd)
	return cmd
}
