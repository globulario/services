#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage:
  cluster-controller-release-guard.sh detect \
    --services-root <path> \
    --packages-root <path> \
    --prev-tag <tag> \
    --output <file>

  cluster-controller-release-guard.sh verify \
    --manifest <change-manifest.json> \
    --forced <forced-packages.txt>
EOF
  exit 1
}

MODE="${1:-}"
[[ -n "${MODE}" ]] || usage
shift || true

SERVICES_ROOT=""
PACKAGES_ROOT=""
PREV_TAG=""
OUTPUT=""
MANIFEST=""
FORCED=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --services-root) SERVICES_ROOT="$2"; shift 2 ;;
    --packages-root) PACKAGES_ROOT="$2"; shift 2 ;;
    --prev-tag) PREV_TAG="$2"; shift 2 ;;
    --output) OUTPUT="$2"; shift 2 ;;
    --manifest) MANIFEST="$2"; shift 2 ;;
    --forced) FORCED="$2"; shift 2 ;;
    *) usage ;;
  esac
done

collect_changed_paths() {
  local repo_root="$1"
  local tag="$2"
  shift 2
  if [[ -z "${repo_root}" || -z "${tag}" || ! -d "${repo_root}/.git" ]]; then
    return 0
  fi
  if ! git -C "${repo_root}" rev-parse --verify "${tag}" >/dev/null 2>&1; then
    return 0
  fi
  git -C "${repo_root}" diff --name-only "${tag}" HEAD -- "$@"
}

case "${MODE}" in
  detect)
    [[ -n "${SERVICES_ROOT}" && -n "${PACKAGES_ROOT}" && -n "${PREV_TAG}" && -n "${OUTPUT}" ]] || usage

    mapfile -t service_changes < <(collect_changed_paths "${SERVICES_ROOT}" "${PREV_TAG}" \
      golang/cluster_controller/cluster_controller_server \
      golang/cluster_controller/cluster_controllerpb \
      golang/component_catalog \
      golang/node_agent/node_agent_server/grpc_workflow.go \
      golang/node_agent/node_agent_server/operation_tracker.go \
      golang/node_agent/node_agent_server/workflow_day0.go \
      golang/workflow/definitions/day0.bootstrap.yaml \
      scripts/install.sh \
      scripts/release/install-day0.sh)

    mapfile -t package_changes < <(collect_changed_paths "${PACKAGES_ROOT}" "${PREV_TAG}" \
      metadata/cluster-controller)

    mkdir -p "$(dirname "${OUTPUT}")"
    : > "${OUTPUT}"

    if (( ${#service_changes[@]} > 0 || ${#package_changes[@]} > 0 )); then
      {
        printf 'cluster-controller|founding-profile/bootstrap inputs changed'
        if (( ${#service_changes[@]} > 0 )); then
          printf ' services:%s' "$(IFS=,; echo "${service_changes[*]}")"
        fi
        if (( ${#package_changes[@]} > 0 )); then
          printf ' packages:%s' "$(IFS=,; echo "${package_changes[*]}")"
        fi
        printf '\n'
      } >> "${OUTPUT}"
      echo "cluster-controller release guard: forcing rebuild due to curated input changes"
      printf '  %s\n' "${service_changes[@]}" "${package_changes[@]}" | sed '/^$/d'
    else
      echo "cluster-controller release guard: no curated input changes detected"
    fi
    ;;

  verify)
    [[ -n "${MANIFEST}" && -n "${FORCED}" ]] || usage
    [[ -f "${MANIFEST}" && -f "${FORCED}" ]] || exit 0

    python3 - "${MANIFEST}" "${FORCED}" <<'PY'
import json, sys

manifest_path, forced_path = sys.argv[1:]
manifest = json.load(open(manifest_path))
forced = {}
for raw in open(forced_path):
    line = raw.strip()
    if not line or line.startswith("#"):
        continue
    parts = line.split("|", 1)
    forced[parts[0].strip()] = parts[1].strip() if len(parts) > 1 else ""

packages = {p["name"]: p for p in manifest.get("packages", [])}
errors = []
for name, reason in forced.items():
    pkg = packages.get(name)
    if not pkg:
        errors.append(f"{name}: missing from change-manifest")
        continue
    if not pkg.get("changed"):
        errors.append(f"{name}: forced by guard but marked changed=false ({reason})")
        continue
    if pkg.get("origin_release") != manifest.get("release_tag"):
        errors.append(
            f"{name}: forced by guard but origin_release={pkg.get('origin_release')} "
            f"!= current {manifest.get('release_tag')}"
        )
if errors:
    for err in errors:
        print(f"ERROR: {err}", file=sys.stderr)
    sys.exit(1)
print("cluster-controller release guard: manifest verification passed")
PY
    ;;

  *)
    usage
    ;;
esac
