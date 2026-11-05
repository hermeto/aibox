package doctor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hermeto/aibox/internal/exec"
)

// toolReq is a required tool with a minimum version.
type toolReq struct {
	ID          string
	Names       []string // alternatives, first found wins (e.g. tofu, terraform)
	Min         []string // minimum version per alternative
	VersionArgs [][]string
	Brew        string
	Linux       string
}

var kindTools = []toolReq{
	{ID: "docker", Names: []string{"docker"}, Min: []string{"24.0.0"}, VersionArgs: [][]string{{"--version"}},
		Brew: "brew install --cask docker (or: brew install colima docker && colima start --cpu 6 --memory 12)", Linux: "https://docs.docker.com/engine/install/"},
	{ID: "kind", Names: []string{"kind"}, Min: []string{"0.27.0"}, VersionArgs: [][]string{{"version"}},
		Brew: "brew install kind", Linux: "go install sigs.k8s.io/kind@latest (or https://kind.sigs.k8s.io/docs/user/quick-start/#installation)"},
	{ID: "kubectl", Names: []string{"kubectl"}, Min: []string{"1.31.0"}, VersionArgs: [][]string{{"version", "--client"}},
		Brew: "brew install kubectl", Linux: "https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"},
	{ID: "helm", Names: []string{"helm"}, Min: []string{"3.15.0"}, VersionArgs: [][]string{{"version", "--short"}},
		Brew: "brew install helm", Linux: "https://helm.sh/docs/intro/install/"},
}

var awsTools = []toolReq{
	{ID: "terraform", Names: []string{"tofu", "terraform"}, Min: []string{"1.8.0", "1.9.0"}, VersionArgs: [][]string{{"version"}, {"version"}},
		Brew: "brew install opentofu", Linux: "https://opentofu.org/docs/intro/install/"},
	kindTools[2], // kubectl
	kindTools[3], // helm
}

var versionRE = regexp.MustCompile(`v?(\d+)\.(\d+)(?:\.(\d+))?`)

// parseVersion extracts the first semantic version in s.
func parseVersion(s string) ([3]int, bool) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, false
	}
	var v [3]int
	for i := 1; i <= 3; i++ {
		if m[i] != "" {
			v[i-1], _ = strconv.Atoi(m[i])
		}
	}
	return v, true
}

func versionLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func fmtVersion(v [3]int) string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

func (t toolReq) hint(goos string) string {
	if goos == "darwin" {
		return t.Brew
	}
	return t.Linux
}

func toolChecks(ctx context.Context, env Env, reqs []toolReq) []Check {
	out := make([]Check, 0, len(reqs))
	for _, t := range reqs {
		out = append(out, toolCheck(ctx, env, t))
	}
	return out
}

func toolCheck(ctx context.Context, env Env, t toolReq) Check {
	c := Check{ID: "tool." + t.ID, Name: "tool " + strings.Join(t.Names, "|")}
	for i, name := range t.Names {
		if _, err := env.Runner.LookPath(name); err != nil {
			continue
		}
		minV, _ := parseVersion(t.Min[i])
		res, err := env.Runner.Run(ctx, exec.Cmd{Name: name, Args: t.VersionArgs[i]})
		got, ok := parseVersion(res.Stdout + "\n" + res.Stderr)
		if err != nil || !ok {
			c.Status = Warn
			c.Message = fmt.Sprintf("%s found but its version could not be determined", name)
			c.Remediation = fmt.Sprintf("ensure `%s %s` works; minimum %s", name, strings.Join(t.VersionArgs[i], " "), t.Min[i])
			return c
		}
		c.Details = map[string]string{"tool": name, "version": fmtVersion(got), "minimum": fmtVersion(minV)}
		if versionLess(got, minV) {
			c.Status = Fail
			c.Message = fmt.Sprintf("%s %s is older than the minimum %s", name, fmtVersion(got), fmtVersion(minV))
			c.Remediation = "upgrade: " + t.hint(env.GOOS)
			return c
		}
		c.Status = Pass
		c.Message = fmt.Sprintf("%s %s (>= %s)", name, fmtVersion(got), fmtVersion(minV))
		return c
	}
	c.Status = Fail
	c.Message = "not found in PATH"
	c.Remediation = "install: " + t.hint(env.GOOS)
	return c
}
