#!/usr/bin/env bash
# Renders every platform/ source as one YAML stream: the bootstrap chart for each profile and
# every component overlay (Kustomize with --enable-helm). Used by validate-manifests and the
# image digest check. Env: HELM, KUSTOMIZE (binaries).
# Usage: render-platform.sh            render everything
#        render-platform.sh --list     print the render targets
#        render-platform.sh <target>   render one target (as printed by --list)
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
helm="${HELM:-helm}"
kustomize="${KUSTOMIZE:-kustomize}"

targets() {
  for values in platform/bootstrap/values-*.yaml; do
    echo "$values"
  done
  for component in platform/*/; do
    [[ "$component" == platform/bootstrap/ ]] && continue
    if [[ -d "${component}overlays" ]]; then
      printf '%s\n' "${component}"overlays/*/
    else
      echo "${component}base/" # components shared by every profile
    fi
  done
}

render() {
  echo "--- # $1" >&2
  if [[ "$1" == platform/bootstrap/values-*.yaml ]]; then
    "$helm" template aibox-root platform/bootstrap -f "$1" \
      --set repoURL=https://github.com/hermeto/aibox.git --set targetRevision=main --set runId=render-check
  else
    "$kustomize" build --enable-helm "$1"
  fi
  echo "---"
}

case "${1:-}" in
  --list) targets ;;
  "") for target in $(targets); do render "$target"; done ;;
  *) render "$1" ;;
esac
