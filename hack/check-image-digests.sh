#!/usr/bin/env bash
# Fails if any image in the rendered platform is not pinned by digest, printing the digest to
# pin (resolved with `docker buildx imagetools inspect`). Constitution principle II.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
status=0
while IFS= read -r image; do
  [[ -z "$image" ]] && continue
  if [[ "$image" == *@sha256:* ]]; then
    echo "ok      $image"
    continue
  fi
  digest="$(docker buildx imagetools inspect "$image" 2>/dev/null | awk '/^Digest:/ {print $2; exit}' || true)"
  echo "::error::image not pinned by digest: ${image} → pin ${image}@${digest:-<unresolved>}"
  status=1
done < <(hack/render-platform.sh 2>/dev/null | go run ./hack/imagelist)
exit "$status"
