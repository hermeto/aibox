package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/cost"
	"github.com/hermeto/aibox/internal/doctor"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/output"
	"github.com/hermeto/aibox/internal/platform"
)

// RoadmapURL is where unimplemented commands point to.
const RoadmapURL = "https://github.com/hermeto/aibox/issues?q=is%3Aissue+%22v1.0+roadmap%22"

// Options are the global flags.
type Options struct {
	Profile string
	Output  string
	Verbose bool
	DryRun  bool
	Yes     bool
}

// App holds the dependencies of the command tree; tests replace them with fakes.
type App struct {
	In     io.Reader // confirmation prompts (nil means os.Stdin)
	Out    io.Writer
	Err    io.Writer
	Runner exec.Runner         // nil means exec.Real
	AWS    doctor.AWSFactory   // nil means the SDK-backed client
	Getenv func(string) string // nil means os.Getenv
	// Kube connects to a kubeconfig context (nil means client-go).
	Kube func(kubeContext string) (platform.KubeAPI, error)
	// KubeFile connects with an explicit kubeconfig file (nil means client-go).
	KubeFile func(kubeconfig, kubeContext string) (platform.KubeAPI, error)
	// Account returns the AWS account ID (nil means sts:GetCallerIdentity).
	Account func(ctx context.Context, region string) (string, error)
	// EgressIP returns the operator's public IP (nil means checkip.amazonaws.com).
	EgressIP func(ctx context.Context) (string, error)
	// CostClients returns read-only EC2/ELBv2 clients (nil means the AWS SDK).
	CostClients func(ctx context.Context, region string) (cost.EC2API, cost.ELBAPI, error)

	Opts   Options
	Format output.Format
	Logger *slog.Logger
}

// runner returns the command runner honoring --dry-run.
func (a *App) runner() exec.Runner {
	base := a.Runner
	if base == nil {
		base = exec.Real{Logger: a.Logger}
	}
	if a.Opts.DryRun {
		return exec.DryRun{Inner: base, Out: a.Err, ReadOnly: exec.ReadOnly}
	}
	return base
}

// readOnlyRunner ignores --dry-run (for commands that never mutate anything).
func (a *App) readOnlyRunner() exec.Runner {
	if a.Runner != nil {
		return a.Runner
	}
	return exec.Real{Logger: a.Logger}
}

func (a *App) getenv(k string) string {
	if a.Getenv != nil {
		return a.Getenv(k)
	}
	return os.Getenv(k)
}

// print renders v in the selected output format.
func (a *App) print(v any) error { return output.Print(a.Out, a.Format, v) }

// NewRootCmd builds the command tree.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "aibox",
		Short: "AIBox — AI Platform in a Box",
		Long: `aibox brings up a reference LLM inference platform on Kubernetes (locally on kind
or on AWS EKS), runs reproducible benchmarks and reports cost per 1M tokens and latency.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			f, err := output.Parse(app.Opts.Output)
			if err != nil {
				return UsageError(err)
			}
			app.Format = f
			level := slog.LevelInfo
			if app.Opts.Verbose {
				level = slog.LevelDebug
			}
			app.Logger = slog.New(slog.NewTextHandler(app.Err, &slog.HandlerOptions{Level: level}))
			app.Logger.Debug("starting", "command", cmd.CommandPath(), "profile", app.Opts.Profile, "dryRun", app.Opts.DryRun)
			return nil
		},
	}
	root.SetOut(app.Out)
	root.SetErr(app.Err)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return UsageError(err) })
	root.CompletionOptions.HiddenDefaultCmd = true

	pf := root.PersistentFlags()
	pf.StringVar(&app.Opts.Profile, "profile", "local", "profile name (profiles/<name>.yaml) or path to a profile file")
	pf.StringVarP(&app.Opts.Output, "output", "o", "text", "output format: text or json")
	pf.BoolVarP(&app.Opts.Verbose, "verbose", "v", false, "verbose (debug) logging on stderr")
	pf.BoolVar(&app.Opts.DryRun, "dry-run", false, "print external commands instead of running them")
	pf.BoolVarP(&app.Opts.Yes, "yes", "y", false, "assume yes for confirmations")

	root.AddCommand(newDoctorCmd(app), newVersionCmd(app), newUpCmd(app), newDownCmd(app), newSmokeCmd(app), newCostCmd(app), newTTLCmd(app), newRenderCmd(app))
	for _, c := range notImplemented {
		root.AddCommand(newNotImplementedCmd(c))
	}
	return root
}

// Execute runs the CLI with args and returns the process exit code.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	app := &App{Out: stdout, Err: stderr}
	return run(ctx, app, args)
}

func run(ctx context.Context, app *App, args []string) int {
	root := NewRootCmd(app)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	code := ExitCode(err)
	if err != nil && !isSilent(err) {
		_, _ = fmt.Fprintf(app.Err, "Error: %v\n", err)
		if app.Format == output.JSON || (app.Format == "" && app.Opts.Output == string(output.JSON)) {
			_ = output.Print(app.Out, output.JSON, map[string]any{"error": err.Error(), "code": code})
		}
		if code == ExitUsage {
			_, _ = fmt.Fprintln(app.Err, "Run 'aibox --help' for usage.")
		}
	}
	return code
}

func goos() string { return runtime.GOOS }

func (a *App) kube() func(string) (platform.KubeAPI, error) {
	if a.Kube != nil {
		return a.Kube
	}
	return func(kubeContext string) (platform.KubeAPI, error) {
		c, err := kube.NewForContext(kubeContext)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
}
