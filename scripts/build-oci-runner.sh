#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-${ROOT}/dist/.staging/bin/globular-oci-runner}"
mkdir -p "$(dirname "${OUT}")"

cd "${ROOT}/golang"
GOWORK=off go build -trimpath -o "${OUT}" ./oci/cmd/globular-oci-runner
printf 'built %s\n' "${OUT}"
