#!/usr/bin/env bash
# Aggregates required job results for the `ci-gate` status check.
# Usage: ci-gate.sh '<toJSON(needs)>' [comma-separated jobs allowed to be skipped]
set -euo pipefail

needs_json="${1:?usage: ci-gate.sh '<needs-json>' [allowed-skips]}"
allowed_skips=",${2:-},"

failed=0
printf '%-24s %s\n' "JOB" "RESULT"
while IFS=$'\t' read -r job result; do
  printf '%-24s %s\n' "$job" "$result"
  if [[ "$result" == "success" ]]; then
    continue
  fi
  if [[ "$result" == "skipped" && "$allowed_skips" == *",$job,"* ]]; then
    continue
  fi
  failed=1
done < <(jq -r 'to_entries[] | [.key, .value.result] | @tsv' <<<"$needs_json")

if [[ "$failed" -ne 0 ]]; then
  echo "ci-gate: at least one required job did not succeed" >&2
  exit 1
fi
echo "ci-gate: all required jobs succeeded"
