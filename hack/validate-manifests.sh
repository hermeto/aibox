#!/usr/bin/env bash
# Renders every platform/ source (bootstrap chart per profile, every overlay) and
# validates the result with kubeconform against Kubernetes and pinned CRD schemas.
# Usage: hack/validate-manifests.sh <kubeconform> <kubernetes-version>
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

kubeconform="${1:?kubeconform binary}"
k8s="${2:?kubernetes version}"
helm="${HELM:-helm}"
kustomize="${KUSTOMIZE:-kustomize}"

render() {
  echo "--- $1" >&2
  shift
  "$@"
}

{
  for values in platform/bootstrap/values-*.yaml; do
    render "bootstrap $(basename "$values")" "$helm" template aibox-root platform/bootstrap -f "$values" \
      --set repoURL=https://github.com/hermeto/aibox.git --set targetRevision=main
    echo "---"
  done
  for overlay in platform/*/overlays/*/; do
    render "$overlay" "$kustomize" build --enable-helm "$overlay"
    echo "---"
  done
} | "$kubeconform" -strict -summary -kubernetes-version "$k8s" \
      -schema-location default \
      -schema-location 'hack/schemas/{{ .Group }}/{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json' \
      -
