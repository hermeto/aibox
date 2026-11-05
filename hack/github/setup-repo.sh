#!/usr/bin/env bash
# One-time repository configuration for the maintainer (requires admin rights and an
# authenticated `gh` with scopes repo, workflow, read:org). Idempotent: safe to re-run.
# Usage: hack/github/setup-repo.sh [owner/repo]
set -euo pipefail

repo="${1:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
echo "Configuring ${repo}"

gh repo edit "$repo" \
  --description "Reference LLM inference platform on Kubernetes — vLLM/SGLang, llm-d, KServe, KEDA, Karpenter, OpenTelemetry — with reproducible cost-per-token and latency benchmarks." \
  --enable-issues --enable-discussions --enable-wiki=false --enable-projects=false \
  --enable-squash-merge --enable-merge-commit=false --enable-rebase-merge=false \
  --delete-branch-on-merge --enable-auto-merge \
  --add-topic kubernetes,llm,llm-inference,vllm,sglang,llm-d,kserve,keda,karpenter,gateway-api,opentelemetry,grafana,gpu,platform-engineering,benchmark,mlops

# Security features
gh api -X PUT "repos/${repo}/vulnerability-alerts" >/dev/null
gh api -X PUT "repos/${repo}/automated-security-fixes" >/dev/null
gh api -X PUT "repos/${repo}/private-vulnerability-reporting" >/dev/null
gh api -X PATCH "repos/${repo}" --input - >/dev/null <<'JSON'
{"security_and_analysis":{"secret_scanning":{"status":"enabled"},"secret_scanning_push_protection":{"status":"enabled"}}}
JSON

# Ruleset for the default branch (create or update by name)
existing="$(gh api "repos/${repo}/rulesets" -q '.[] | select(.name=="main") | .id' || true)"
if [[ -n "$existing" ]]; then
  gh api -X PUT "repos/${repo}/rulesets/${existing}" --input "${root}/.github/rulesets/main.json" >/dev/null
else
  gh api -X POST "repos/${repo}/rulesets" --input "${root}/.github/rulesets/main.json" >/dev/null
fi

# Labels: remove overlapping GitHub defaults, then create/update the catalog
for default in bug documentation enhancement question invalid duplicate wontfix; do
  gh label delete "$default" --repo "$repo" --yes 2>/dev/null || true
done
python3 - "${root}/.github/labels.yml" <<'PY' | while IFS=$'\t' read -r name color desc; do
import sys, re
name = color = desc = None
for line in open(sys.argv[1]):
    m = re.match(r'\s*-?\s*(name|color|description):\s*"(.*)"\s*$', line)
    if not m:
        continue
    k, v = m.groups()
    if k == "name": name = v
    elif k == "color": color = v
    else:
        print(f"{name}\t{color}\t{v}")
PY
  gh label create "$name" --repo "$repo" --color "$color" --description "$desc" --force >/dev/null
done

# Milestones
for m in "M0 Foundation" "M1 First token" "M2 Observability" "M3 Routing + autoscaling" "M4 Benchmark harness" "M5 Report v1.0"; do
  if ! gh api "repos/${repo}/milestones?state=all" -q '.[].title' | grep -Fxq "$m"; then
    gh api -X POST "repos/${repo}/milestones" -f title="$m" >/dev/null
  fi
done

echo "Done. Remaining manual steps: pin the 'v1.0 roadmap' issue; rename the stand-in default branch to 'main' if it is not already."
