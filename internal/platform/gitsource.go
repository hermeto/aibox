// Package platform orchestrates bring-up and teardown of AIBox environments.
package platform

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hermeto/aibox/internal/exec"
)

// GitSource is the repository and revision Argo CD tracks.
type GitSource struct {
	RepoURL  string `json:"repoURL"`
	Revision string `json:"revision"`
	// Unpushed is the number of local commits not on the upstream branch (-1 if unknown).
	Unpushed int `json:"unpushedCommits"`
	// Warning explains why Argo CD may not see the local state.
	Warning string `json:"warning,omitempty"`
}

var (
	scpLike = regexp.MustCompile(`^(?:[\w.-]+@)?([\w.-]+):(.+?)(?:\.git)?/?$`)
	sshURL  = regexp.MustCompile(`^ssh://(?:[\w.-]+@)?([\w.-]+)(?::\d+)?/(.+?)(?:\.git)?/?$`)
	httpURL = regexp.MustCompile(`^https?://(?:[^@/]+@)?([\w.-]+(?::\d+)?)/(.+?)(?:\.git)?/?$`)
)

// NormalizeRepoURL converts SSH and HTTPS remotes to "https://host/owner/repo.git" and
// strips embedded credentials.
func NormalizeRepoURL(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	for _, re := range []*regexp.Regexp{sshURL, httpURL} {
		if m := re.FindStringSubmatch(remote); m != nil {
			return fmt.Sprintf("https://%s/%s.git", m[1], m[2]), nil
		}
	}
	if !strings.Contains(remote, "://") {
		if m := scpLike.FindStringSubmatch(remote); m != nil {
			return fmt.Sprintf("https://%s/%s.git", m[1], m[2]), nil
		}
	}
	return "", fmt.Errorf("unsupported git remote URL %q (use --repo-url)", remote)
}

// DetectGitSource resolves the repository URL and revision from the working tree. Explicit
// values win; the remote defaults to "origin" and the revision to the current branch (or the
// commit SHA on a detached HEAD).
func DetectGitSource(ctx context.Context, r exec.Runner, dir, repoURL, revision string) (GitSource, error) {
	src := GitSource{RepoURL: repoURL, Revision: revision, Unpushed: -1}
	git := func(args ...string) (string, error) {
		res, err := r.Run(ctx, exec.Cmd{Name: "git", Args: args, Dir: dir})
		return strings.TrimSpace(res.Stdout), err
	}
	if src.RepoURL == "" {
		remote, err := git("remote", "get-url", "origin")
		if err != nil || remote == "" {
			return src, fmt.Errorf("cannot determine the git remote (use --repo-url or --from-worktree): %w", err)
		}
		if src.RepoURL, err = NormalizeRepoURL(remote); err != nil {
			return src, err
		}
	}
	if src.Revision == "" {
		branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return src, fmt.Errorf("cannot determine the current branch (use --revision): %w", err)
		}
		if branch == "HEAD" { // detached
			if branch, err = git("rev-parse", "HEAD"); err != nil {
				return src, err
			}
		}
		src.Revision = branch
		if count, err := git("rev-list", "--count", "@{u}..HEAD"); err == nil {
			if n, convErr := strconv.Atoi(count); convErr == nil {
				src.Unpushed = n
				if n > 0 {
					src.Warning = fmt.Sprintf("%d local commit(s) are not pushed; Argo CD only sees %s on the remote (push, or use --from-worktree)", n, src.Revision)
				}
			}
		} else if !isSHA(branch) {
			src.Warning = fmt.Sprintf("branch %s has no upstream; push it or use --from-worktree", branch)
		}
	}
	return src, nil
}

var shaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

func isSHA(s string) bool { return shaRE.MatchString(s) }
