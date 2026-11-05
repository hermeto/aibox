#!/usr/bin/env bash
# Verifies the Developer Certificate of Origin sign-off on every non-merge commit in
# <base>..<head>. Each commit must carry a "Signed-off-by:" trailer whose email matches
# the commit author's email. Usage: dco-check.sh <base-ref> <head-ref>
set -euo pipefail

base="${1:?usage: dco-check.sh <base-ref> <head-ref>}"
head="${2:?usage: dco-check.sh <base-ref> <head-ref>}"

failed=0
checked=0
while IFS= read -r sha; do
  [[ -z "$sha" ]] && continue
  checked=$((checked + 1))
  author_email="$(git show -s --format='%ae' "$sha")"
  subject="$(git show -s --format='%s' "$sha")"
  if git show -s --format='%B' "$sha" \
      | grep -Eiq "^Signed-off-by: .+ <${author_email//./\\.}>\s*$"; then
    echo "ok    ${sha:0:12} ${subject}"
  else
    echo "FAIL  ${sha:0:12} ${subject}"
    echo "      missing 'Signed-off-by: <name> <${author_email}>' (use: git commit -s)"
    failed=$((failed + 1))
  fi
done < <(git rev-list --no-merges "${base}..${head}")

echo "DCO: checked ${checked} commit(s), ${failed} without a valid sign-off."
if [[ "$failed" -ne 0 ]]; then
  echo "Fix with: git rebase --signoff ${base} && git push --force-with-lease" >&2
  exit 1
fi
