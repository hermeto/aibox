#!/usr/bin/env bash
# Validates the rendered platform (hack/render-platform.sh) with kubeconform against the
# Kubernetes schemas and the CRD schemas generated from pinned charts (hack/schemas).
# Usage: hack/validate-manifests.sh <kubeconform> <kubernetes-version>
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
kubeconform="${1:?kubeconform binary}"
k8s="${2:?kubernetes version}"
hack/render-platform.sh | "$kubeconform" -strict -summary -kubernetes-version "$k8s" \
  -schema-location default \
  -schema-location 'hack/schemas/{{ .Group }}/{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json' \
  -
