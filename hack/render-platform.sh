#!/usr/bin/env bash
# Renders every platform/ source as one YAML stream: the bootstrap chart for each profile and
# every component overlay (Kustomize with --enable-helm). Used by validate-manifests and the
# image digest check. Env: HELM, KUSTOMIZE (binaries).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
helm="${HELM:-helm}"
kustomize="${KUSTOMIZE:-kustomize}"

for values in platform/bootstrap/values-*.yaml; do
  echo "--- # bootstrap $(basename "$values")" >&2
  "$helm" template aibox-root platform/bootstrap -f "$values" \
    --set repoURL=https://github.com/hermeto/aibox.git --set targetRevision=main --set runId=render-check
  echo "---"
done
for overlay in platform/*/overlays/*/; do
  echo "--- # $overlay" >&2
  "$kustomize" build --enable-helm "$overlay"
  echo "---"
done
