#!/usr/bin/env bash
# Tests for dco-check.sh using a throwaway git repository.
set -euo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/dco-check.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

git init -q -b main .
git config user.name "Test User"
git config user.email "test@example.com"
git config commit.gpgsign false
git commit -q --allow-empty -m "chore: base"
base="$(git rev-parse HEAD)"

pass=0
fail=0
expect() {
  local want="$1" name="$2"
  if "$script" "$base" HEAD >/dev/null 2>&1; then got=0; else got=1; fi
  if [[ "$got" == "$want" ]]; then
    pass=$((pass + 1)); echo "PASS $name"
  else
    fail=$((fail + 1)); echo "FAIL $name (exit $got, want $want)"
  fi
}

git checkout -q -b signed
git commit -q --allow-empty -s -m "feat: signed commit"
expect 0 "signed commit passes"

git commit -q --allow-empty -m "feat: unsigned commit"
expect 1 "unsigned commit fails"

git checkout -q -B mismatch "$base"
git commit -q --allow-empty -m "feat: wrong email

Signed-off-by: Test User <other@example.com>"
expect 1 "sign-off with a different email fails"

git checkout -q -B merge "$base"
git checkout -q -b side
git commit -q --allow-empty -s -m "feat: side"
git checkout -q merge
git merge -q --no-ff --no-edit side
expect 0 "unsigned merge commit is ignored"

echo "dco-check tests: ${pass} passed, ${fail} failed"
[[ "$fail" -eq 0 ]]
