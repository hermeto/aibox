package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hermeto/aibox/internal/awsapi"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/platform"
	"github.com/hermeto/aibox/internal/profile"
)

// confirm prints prompt and reads "yes"/"y" from stdin.
func (a *App) confirm(prompt string) (bool, error) {
	in := a.In
	if in == nil {
		in = os.Stdin
	}
	if _, err := fmt.Fprintf(a.Err, "%s [y/N]: ", prompt); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("no answer on stdin (use --yes): %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func (a *App) awsPlatform(p *profile.Profile) (*platform.AWS, error) {
	root, err := profile.RepoRoot()
	if err != nil {
		return nil, fmt.Errorf("run aibox from a checkout of the AIBox repository: %w", err)
	}
	account := a.Account
	if account == nil {
		account = func(ctx context.Context, region string) (string, error) {
			c, err := awsapi.New(ctx, region)
			if err != nil {
				return "", err
			}
			acct, _, err := c.CallerIdentity(ctx)
			return acct, err
		}
	}
	egress := a.EgressIP
	if egress == nil {
		egress = platform.CheckIPEgress(&http.Client{Timeout: 10 * time.Second})
	}
	kubeFile := a.KubeFile
	if kubeFile == nil {
		kubeFile = func(kubeconfig, kubeContext string) (platform.KubeAPI, error) {
			c, err := kube.NewForKubeconfig(kubeconfig, kubeContext)
			if err != nil {
				return nil, err
			}
			return c, nil
		}
	}
	region := p.Spec.AWS.Region
	stateDir := ""
	if d := a.getenv("AIBOX_STATE_DIR"); d != "" {
		stateDir = filepath.Join(d, p.Metadata.Name)
	}
	return &platform.AWS{
		Profile:  p,
		Runner:   a.runner(),
		Root:     root,
		StateDir: stateDir,
		Log:      a.Err,
		DryRun:   a.Opts.DryRun,
		Yes:      a.Opts.Yes,
		Confirm:  a.confirm,
		Account:  func(ctx context.Context) (string, error) { return account(ctx, region) },
		EgressIP: egress,
		Kube:     kubeFile,
		Getenv:   a.getenv,
	}, nil
}
