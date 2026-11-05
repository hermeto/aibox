package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
)

type dockerInfo struct {
	NCPU          int    `json:"NCPU"`
	MemTotal      int64  `json:"MemTotal"`
	ServerVersion string `json:"ServerVersion"`
	Architecture  string `json:"Architecture"`
}

const gib = 1 << 30

func dockerChecks(ctx context.Context, env Env, local *profile.Local) []Check {
	daemon := Check{ID: "docker.daemon", Name: "docker daemon"}
	res, err := env.Runner.Run(ctx, exec.Cmd{Name: "docker", Args: []string{"info", "--format", "{{json .}}"}})
	var info dockerInfo
	if err == nil {
		err = json.Unmarshal([]byte(res.Stdout), &info)
	}
	if err != nil || info.ServerVersion == "" {
		daemon.Status = Fail
		daemon.Message = "cannot reach the Docker daemon"
		daemon.Remediation = "start Docker (Docker Desktop, `colima start --cpu 6 --memory 12`, or `sudo systemctl start docker`)"
		skip := func(id, name string) Check {
			return Check{ID: id, Name: name, Status: Skip, Message: "skipped: Docker daemon unreachable"}
		}
		return []Check{daemon, skip("docker.cpus", "docker CPUs"), skip("docker.memory", "docker memory")}
	}
	daemon.Status = Pass
	daemon.Message = fmt.Sprintf("server %s (%s)", info.ServerVersion, info.Architecture)
	daemon.Details = map[string]string{"serverVersion": info.ServerVersion, "architecture": info.Architecture}

	if local == nil {
		local = &profile.Local{MinCPUs: 1, MinMemoryGiB: 1, RecommendedMemoryGiB: 1}
	}
	cpus := Check{ID: "docker.cpus", Name: "docker CPUs", Details: map[string]string{"cpus": strconv.Itoa(info.NCPU), "minimum": strconv.Itoa(local.MinCPUs)}}
	if info.NCPU < local.MinCPUs {
		cpus.Status = Fail
		cpus.Message = fmt.Sprintf("%d CPUs available, %d required", info.NCPU, local.MinCPUs)
		cpus.Remediation = fmt.Sprintf("give the Docker VM at least %d CPUs (Docker Desktop settings or `colima start --cpu %d`)", local.MinCPUs, local.MinCPUs)
	} else {
		cpus.Status = Pass
		cpus.Message = fmt.Sprintf("%d CPUs (>= %d)", info.NCPU, local.MinCPUs)
	}

	memGiB := float64(info.MemTotal) / gib
	mem := Check{ID: "docker.memory", Name: "docker memory", Details: map[string]string{
		"memoryGiB": strconv.FormatFloat(memGiB, 'f', 1, 64), "minimumGiB": strconv.FormatFloat(local.MinMemoryGiB, 'f', 1, 64),
		"recommendedGiB": strconv.FormatFloat(local.RecommendedMemoryGiB, 'f', 1, 64),
	}}
	switch {
	case memGiB < local.MinMemoryGiB:
		mem.Status = Fail
		mem.Message = fmt.Sprintf("%.1f GiB available, %.0f GiB required", memGiB, local.MinMemoryGiB)
		mem.Remediation = fmt.Sprintf("give the Docker VM at least %.0f GiB of memory", local.RecommendedMemoryGiB)
	case memGiB < local.RecommendedMemoryGiB:
		mem.Status = Warn
		mem.Message = fmt.Sprintf("%.1f GiB available, %.0f GiB recommended", memGiB, local.RecommendedMemoryGiB)
		mem.Remediation = fmt.Sprintf("give the Docker VM %.0f GiB of memory for the full platform", local.RecommendedMemoryGiB)
	default:
		mem.Status = Pass
		mem.Message = fmt.Sprintf("%.1f GiB (>= %.0f GiB recommended)", memGiB, local.RecommendedMemoryGiB)
	}
	return []Check{daemon, cpus, mem}
}
