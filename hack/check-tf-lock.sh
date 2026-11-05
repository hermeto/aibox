#!/usr/bin/env bash
# Fails if `tofu init` changed or created a provider lock file, printing it so the exact
# registry hashes can be committed. Usage: hack/check-tf-lock.sh <dir>...
set -euo pipefail
status=0
for dir in "$@"; do
  lock="$dir/.terraform.lock.hcl"
  if ! git ls-files --error-unmatch "$lock" >/dev/null 2>&1 || ! git diff --quiet -- "$lock"; then
    echo "::error file=$lock::provider lock file missing or out of date; commit this content:"
    cat "$lock"
    status=1
  fi
done
exit "$status"
