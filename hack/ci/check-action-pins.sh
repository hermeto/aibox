#!/usr/bin/env bash
# Fails if a workflow step uses a third-party action that is not pinned to a full
# 40-character commit SHA followed by a version comment, e.g.
#   uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
# Local actions (./path) and docker:// references with a digest are allowed.
set -euo pipefail

root="${1:-.github}"
bad=0
while IFS= read -r line; do
  file="${line%%:*}"
  rest="${line#*:}"; lineno="${rest%%:*}"; content="${rest#*:}"
  ref="$(sed -E 's/^[[:space:]-]*uses:[[:space:]]*//; s/[[:space:]]+#.*$//; s/["'\'']//g' <<<"$content")"
  case "$ref" in
    ./*) continue ;;
    docker://*@sha256:*) continue ;;
  esac
  if [[ ! "$ref" =~ @[0-9a-f]{40}$ ]] || [[ ! "$content" =~ \#[[:space:]]*v?[0-9] ]]; then
    echo "${file}:${lineno}: not pinned to a commit SHA with a version comment: ${ref}"
    bad=1
  fi
done < <(grep -rnE '^[[:space:]-]*uses:' "$root" --include='*.yml' --include='*.yaml' || true)

if [[ "$bad" -ne 0 ]]; then
  exit 1
fi
echo "All actions are pinned by commit SHA."
