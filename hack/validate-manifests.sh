#!/usr/bin/env bash
# Validates the rendered platform (hack/render-platform.sh) with kubeconform against the
# Kubernetes schemas and the CRD schemas generated from pinned charts (hack/schemas).
# Usage: hack/validate-manifests.sh <kubeconform> <kubernetes-version>
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
kubeconform="${1:?kubeconform binary}"
k8s="${2:?kubernetes version}"
status=0
for target in $(hack/render-platform.sh --list); do
  # KServe's CRD chart (platform/kserve/base/crds) is validated by the API server when
  # e2e-local applies it: its 2.6 MB LLMInferenceServiceConfig CRD, as rendered from the
  # published chart, does not parse with kubeconform's YAML reader.
  [[ "$target" == platform/kserve/base/ ]] && target=platform/kserve/base/controller/
  hack/render-platform.sh "$target" | "$kubeconform" -strict -summary -kubernetes-version "$k8s" \
    -schema-location default \
    -schema-location 'hack/schemas/{{ .Group }}/{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json' \
    - || status=1
done
exit "$status"
