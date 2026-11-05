package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/doctor"
	"github.com/hermeto/aibox/internal/output"
	"github.com/hermeto/aibox/internal/platform"
	"github.com/hermeto/aibox/internal/profile"
)

func (a *App) localPlatform(p *profile.Profile) (*platform.Local, error) {
	root, err := profile.RepoRoot()
	if err != nil {
		return nil, errors.New("run aibox from a checkout of the AIBox repository (platform manifests are read from it)")
	}
	return &platform.Local{
		Profile: p,
		Runner:  a.runner(),
		Root:    root,
		Kube:    a.kube(),
		Log:     a.Err,
		DryRun:  a.Opts.DryRun,
	}, nil
}

func newUpCmd(app *App) *cobra.Command {
	var (
		opts        platform.UpOptions
		adminCIDRs  []string
		owner       string
		budgetFlag  float64
		notifyEmail string
		ttl         time.Duration
		skipDoctor  bool
	)
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Bring up the platform for a profile (cluster, GitOps, workloads)",
		Long: `up creates (or reuses) the cluster for the profile, installs Argo CD, applies the
app-of-apps for the profile and waits until every Application is Synced and Healthy.

By default Argo CD tracks the current branch of the origin remote; use --repo-url and
--revision to choose another source, or --from-worktree to apply manifests from the working
tree (unpushed changes, private forks).`,
		Example: "  aibox up --profile local\n  aibox up --profile local --from-worktree\n  aibox up --profile local --revision $(git rev-parse HEAD)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			budget, err := app.budget(budgetFlag)
			if err != nil {
				return UsageError(err)
			}
			if p.IsCloud() && budget <= 0 {
				return platform.ErrBudgetRequired
			}
			if !skipDoctor {
				env := doctor.Env{Runner: app.readOnlyRunner(), Getenv: app.getenv, GOOS: goos(), BudgetUSD: budget, TTL: ttl}
				if p.IsCloud() {
					env.AWS = app.awsFactory()
					env.PriceTable = priceTable(p.Spec.AWS.Region)
				}
				report := doctor.Run(cmd.Context(), p, env)
				if !report.OK || (p.IsCloud() && report.Summary.Skip > 0) {
					_ = output.Print(app.Err, output.Text, report)
					return errors.New("preflight checks failed or were skipped (see above); fix them or pass --skip-doctor")
				}
			}
			if p.IsCloud() {
				a, err := app.awsPlatform(p)
				if err != nil {
					return err
				}
				res, err := a.Up(cmd.Context(), platform.AWSUpOptions{
					RepoURL: opts.RepoURL, Revision: opts.Revision, AdminCIDRs: adminCIDRs, Owner: owner, Timeout: opts.Timeout,
					BudgetUSD: budget, NotifyEmail: notifyEmail, TTL: ttl,
				})
				if err != nil {
					return err
				}
				return app.print(res)
			}
			l, err := app.localPlatform(p)
			if err != nil {
				return err
			}
			res, err := l.Up(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return app.print(res)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.RepoURL, "repo-url", "", "Git repository Argo CD tracks (default: origin remote, HTTPS)")
	f.StringVar(&opts.Revision, "revision", "", "branch, tag or commit Argo CD tracks (default: current branch)")
	f.BoolVar(&opts.FromWorktree, "from-worktree", false, "apply platform manifests from the working tree instead of Argo CD tracking git")
	f.DurationVar(&opts.Timeout, "timeout", 10*time.Minute, "maximum time to wait for Argo CD and Applications")
	f.BoolVar(&skipDoctor, "skip-doctor", false, "skip the preflight checks")
	f.StringSliceVar(&adminCIDRs, "admin-cidr", nil, "AWS: CIDR(s) allowed to reach the EKS API (default: your egress IP /32)")
	f.StringVar(&owner, "owner", "", "AWS: owner tag (default: profile ownerEmail, then git user.email)")
	f.Float64Var(&budgetFlag, "budget", 0, "AWS: run budget in USD (or BUDGET_USD); required for cloud profiles")
	f.StringVar(&notifyEmail, "notify-email", "", "AWS: e-mail for budget and teardown alerts (default: profile ownerEmail)")
	f.DurationVar(&ttl, "ttl", 0, "AWS: automatic teardown after this duration (default: profile guardrails.ttl; 30m-24h)")
	return cmd
}

func newDownCmd(app *App) *cobra.Command {
	var orphansOnly bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Tear down the platform for a profile and check for orphans",
		Long: `down deletes the environment of the profile. For cloud profiles it drains the cluster,
destroys the main stack and then the guardrails stack, and finishes with an orphan check
that lists every resource still tagged with the run ID (exit 1 if any remain).`,
		Example: "  aibox down --profile local\n  aibox down --profile aws-dev\n  aibox down --profile aws-dev --orphans-only",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := profile.Load(app.Opts.Profile)
			if err != nil {
				return err
			}
			if p.IsCloud() {
				a, err := app.awsPlatform(p)
				if err != nil {
					return err
				}
				if orphansOnly {
					report, err := a.CheckOrphans(cmd.Context())
					if report != nil {
						if perr := app.print(report); perr != nil {
							return perr
						}
					}
					return silenceReported(err, report != nil)
				}
				res, err := a.Down(cmd.Context())
				if res != nil {
					if perr := app.print(res); perr != nil {
						return perr
					}
				}
				return silenceReported(err, res != nil && res.Orphans != nil)
			}
			l, err := app.localPlatform(p)
			if err != nil {
				return err
			}
			deleted, err := l.Down(cmd.Context())
			if err != nil {
				return err
			}
			return app.print(downResult{Profile: p.Metadata.Name, Cluster: p.Spec.Cluster.Name, Deleted: deleted})
		},
	}
	cmd.Flags().BoolVar(&orphansOnly, "orphans-only", false, "AWS: only run the orphan check for the current run")
	return cmd
}

// silenceReported marks err as already reported when a report was printed.
func silenceReported(err error, printed bool) error {
	if err == nil {
		return nil
	}
	if printed && errors.Is(err, platform.ErrOrphans) {
		return &ExitError{Code: ExitFailure, Err: err, Silent: true}
	}
	return err
}
