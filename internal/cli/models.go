package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/profile"
)

func newModelsCmd(app *App) *cobra.Command {
	var catalogPath string
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List catalog models and mirror their weights",
		Long: `models reads models/catalog.yaml (validated against schemas/catalog.schema.json).
list prints the catalog; sync mirrors each model's pinned revision from Hugging Face into the
environment's object storage with in-cluster Jobs and verifies the checksums.`,
		Args: cobra.NoArgs,
	}
	cmd.PersistentFlags().StringVar(&catalogPath, "catalog", "", "catalog file (default: models/catalog.yaml in the repository, or $AIBOX_CATALOG)")
	load := func() (*models.Catalog, error) {
		if catalogPath == "" {
			catalogPath = models.DefaultPath()
		}
		return models.Load(catalogPath)
	}

	var all bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List the models of the catalog (all profiles unless --profile is given)",
		Example: "  aibox models list\n  aibox models list --profile aws-bench\n" +
			"  aibox models list --all --output json | jq '.models[].id'",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := load()
			if err != nil {
				return err
			}
			name := ""
			if cmd.Flags().Changed("profile") {
				p, err := profile.Load(app.Opts.Profile)
				if err != nil {
					return err
				}
				name = p.Metadata.Name
			}
			ms, err := c.Select(name, all, nil)
			if err != nil {
				return err
			}
			return app.print(models.List{Profile: name, Models: ms})
		},
	}
	list.Flags().BoolVar(&all, "all", false, "include disabled models")

	var ids []string
	var noVerify bool
	var timeout time.Duration
	sync := &cobra.Command{
		Use:   "sync",
		Short: "Mirror model weights into the environment's object storage and verify them",
		Long: `sync runs one mirror Job per model in the aibox-models namespace: it copies the pinned
revision from Hugging Face to s3://<bucket>/models/<id>/<revision>/ with SHA256SUMS and a
.complete marker (a mirrored model is skipped; an interrupted mirror resumes), then a verify
Job streams the objects back and checks every checksum. Exit code 1 if any model fails.`,
		Example: "  aibox models sync --profile local --model tiny-random-llama\n  aibox models sync --profile aws-bench",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			c, err := load()
			if err != nil {
				return err
			}
			sel, err := c.Select(p.Metadata.Name, false, ids)
			if err != nil {
				return UsageError(err)
			}
			api, target, err := app.modelTarget(p)
			if err != nil {
				return err
			}
			if app.Opts.DryRun {
				return app.print(dryRunSync(p.Metadata.Name, target, sel))
			}
			s := &models.Syncer{API: api, Target: target, Verify: !noVerify, Timeout: timeout, PollInterval: app.pollInterval(), Log: app.Err}
			rep := s.Sync(cmd.Context(), sel)
			rep.Profile = p.Metadata.Name
			if err := app.print(rep); err != nil {
				return err
			}
			if failed := rep.Failed(); len(failed) > 0 {
				return &ExitError{Code: ExitFailure, Err: fmt.Errorf("model sync failed for %s", strings.Join(failed, ", "))}
			}
			return nil
		},
	}
	sync.Flags().StringArrayVar(&ids, "model", nil, "model id to sync (repeatable; default: every enabled model of the profile)")
	sync.Flags().BoolVar(&noVerify, "no-verify", false, "skip the verify Job")
	sync.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "time limit per Job")

	cmd.AddCommand(list, sync)
	return cmd
}

// dryRunSync reports what sync would do without touching the cluster.
func dryRunSync(profileName string, t models.Target, sel []models.Model) *models.Report {
	rep := &models.Report{Profile: profileName, Bucket: t.Bucket, Endpoint: t.Endpoint}
	for i := range sel {
		m := &sel[i]
		msg := "[dry-run] would skip: no weights to mirror"
		if m.Mirrorable() {
			msg = "[dry-run] would run Jobs " + models.JobName(models.KindMirror, m.ID) + " and " + models.JobName(models.KindVerify, m.ID) + " → " + m.Prefix()
		}
		rep.Models = append(rep.Models, models.Result{ID: m.ID, Revision: m.Source.Revision, Status: models.StatusSkipped, Message: msg})
	}
	return rep
}

// modelTarget connects to the profile's cluster and resolves its weights bucket.
func (a *App) modelTarget(p *profile.Profile) (models.JobAPI, models.Target, error) {
	connect := a.Jobs
	if connect == nil {
		connect = func(kubeconfig, kubeContext string) (models.JobAPI, error) {
			return kube.NewForKubeconfig(kubeconfig, kubeContext)
		}
	}
	if !p.IsCloud() {
		l, err := a.localPlatform(p)
		if err != nil {
			return nil, models.Target{}, err
		}
		api, err := connect("", l.KubeContext())
		return api, models.LocalTarget(), err
	}
	aws, err := a.awsPlatform(p)
	if err != nil {
		return nil, models.Target{}, err
	}
	kubeconfig, kubeContext, outs, err := aws.ActiveCluster()
	if err != nil {
		return nil, models.Target{}, err
	}
	bucket, _ := outs["weights_bucket"].(string)
	if bucket == "" {
		return nil, models.Target{}, fmt.Errorf("terraform outputs of profile %s lack weights_bucket", p.Metadata.Name)
	}
	api, err := connect(kubeconfig, kubeContext)
	return api, models.Target{Bucket: bucket, Region: p.Spec.AWS.Region}, err
}
