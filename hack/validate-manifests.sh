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
  extra=()
  # KServe's chart ships upstream CRDs (LLMInferenceService, Inference Extension) that the API
  # server validates when e2e-local applies them; the 2.6 MB LLMInferenceServiceConfig CRD as
  # rendered from the published chart does not parse with kubeconform's YAML reader.
  [[ "$target" == platform/kserve/base/ ]] && extra=(-skip CustomResourceDefinition)
  hack/render-platform.sh "$target" | "$kubeconform" -strict -summary -kubernetes-version "$k8s" \
    -schema-location default \
    -schema-location 'hack/schemas/{{ .Group }}/{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json' \
    "${extra[@]}" - || status=1
done
exit "$status"
