#!/usr/bin/env bash
# Downloads the CRDs of pinned platform components and regenerates hack/schemas/ (strict
# JSON Schemas for kubeconform). Re-run after bumping a component version.
# K8S_VERSION must match K8S_SCHEMA_VERSION in the Makefile.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
k8s="${K8S_VERSION:-1.37.0}"

# name|url (pinned to the component version used in platform/)
crds=(
  "argocd-application|https://raw.githubusercontent.com/argoproj/argo-cd/v3.6.1/manifests/crds/application-crd.yaml"
  "karpenter-nodepools|https://raw.githubusercontent.com/aws/karpenter-provider-aws/v1.15.1/pkg/apis/crds/karpenter.sh_nodepools.yaml"
  "karpenter-ec2nodeclasses|https://raw.githubusercontent.com/aws/karpenter-provider-aws/v1.15.1/pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml"
  "eso-clustersecretstores|https://raw.githubusercontent.com/external-secrets/external-secrets/helm-chart-2.14.0/config/crds/bases/external-secrets.io_clustersecretstores.yaml"
  "eso-externalsecrets|https://raw.githubusercontent.com/external-secrets/external-secrets/helm-chart-2.14.0/config/crds/bases/external-secrets.io_externalsecrets.yaml"
)

mkdir -p hack/crds
for entry in "${crds[@]}"; do
  name="${entry%%|*}"
  url="${entry#*|}"
  curl -fsSL "$url" -o "hack/crds/${name}.yaml"
done
go run ./hack/crd2schema -out hack/schemas hack/crds/*.yaml

# The default kubeconform registry has no schema for CustomResourceDefinition itself, which
# charts that ship CRDs (Karpenter) render; extract it from the Kubernetes OpenAPI document.
curl -fsSL "https://raw.githubusercontent.com/kubernetes/kubernetes/v${k8s}/api/openapi-spec/v3/apis__apiextensions.k8s.io__v1_openapi.json" \
  -o hack/crds/apiextensions-v1-openapi.json
go run ./hack/crd2schema -out hack/schemas -openapi hack/crds/apiextensions-v1-openapi.json -kind CustomResourceDefinition
