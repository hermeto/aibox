#!/usr/bin/env bash
# Downloads the CRDs of pinned platform components and regenerates hack/schemas/ (strict
# JSON Schemas for kubeconform). Re-run after bumping a component version.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# name|url (pinned to the component version used in platform/)
crds=(
  "argocd-application|https://raw.githubusercontent.com/argoproj/argo-cd/v3.6.1/manifests/crds/application-crd.yaml"
  "karpenter-nodepools|https://raw.githubusercontent.com/aws/karpenter-provider-aws/v1.15.1/pkg/apis/crds/karpenter.sh_nodepools.yaml"
  "karpenter-ec2nodeclasses|https://raw.githubusercontent.com/aws/karpenter-provider-aws/v1.15.1/pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml"
)

mkdir -p hack/crds
for entry in "${crds[@]}"; do
  name="${entry%%|*}"
  url="${entry#*|}"
  curl -fsSL "$url" -o "hack/crds/${name}.yaml"
done
go run ./hack/crd2schema -out hack/schemas hack/crds/*.yaml
