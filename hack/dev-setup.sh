#!/usr/bin/env bash
# Checks the prerequisites for AIBox development. Works on macOS and Linux, with or
# without Homebrew. Prints the install command for anything missing.
set -euo pipefail

missing=0
os="$(uname -s)"

hint() {
  local brew_pkg="$1" other="$2"
  if [[ "$os" == "Darwin" ]] && command -v brew >/dev/null 2>&1; then
    echo "    install: brew install ${brew_pkg}"
  else
    echo "    install: ${other}"
  fi
}

check() {
  local tool="$1" required="$2" brew_pkg="$3" other="$4"
  if command -v "$tool" >/dev/null 2>&1; then
    printf '  ok    %-10s %s\n' "$tool" "$("$tool" --version 2>/dev/null | head -n1 || true)"
  elif [[ "$required" == "required" ]]; then
    printf '  FAIL  %-10s missing (required)\n' "$tool"
    hint "$brew_pkg" "$other"
    missing=1
  else
    printf '  warn  %-10s missing (optional for lint/test)\n' "$tool"
    hint "$brew_pkg" "$other"
  fi
}

echo "Checking AIBox development prerequisites ($os)..."
check git    required git    "your package manager (apt install git)"
check go     required go     "https://go.dev/dl/ (version in go.mod)"
check uv     required uv     "curl -LsSf https://astral.sh/uv/install.sh | sh"
check node   required node   "https://nodejs.org/ or your package manager (apt install nodejs npm)"
check make   required make   "apt install make"
check docker optional docker "https://docs.docker.com/engine/install/ (needed from feature 003)"

if [[ "$missing" -ne 0 ]]; then
  echo "Install the missing required tools above, then re-run: make dev-setup" >&2
  exit 1
fi
echo "Prerequisites satisfied. No cloud credentials are needed for lint or test."
