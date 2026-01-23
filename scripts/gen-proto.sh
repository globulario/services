#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT/proto"
GEN_GO="${GEN_GO:-1}"
GEN_TS="${GEN_TS:-1}"
GEN_PY="${GEN_PY:-0}"

die() { echo "ERROR: $*" >&2; exit 1; }

command -v protoc >/dev/null 2>&1 || die "protoc not found in PATH"

check_plugin() {
  local plugin="$1"
  if ! command -v "$plugin" >/dev/null 2>&1; then
    die "missing required plugin: $plugin (set GEN_${plugin^^}=0 to skip)"
  fi
}

if [[ "$GEN_GO" == "1" ]]; then
  check_plugin protoc-gen-go
  check_plugin protoc-gen-go-grpc
fi

declare -A GO_OUT=(
  ["plan"]="$ROOT/golang/plan/planpb"
  ["node_agent"]="$ROOT/golang/nodeagent/nodeagentpb"
  ["clustercontroller"]="$ROOT/golang/clustercontroller/clustercontrollerpb"
)

declare -A TS_OUT=(
  ["plan"]="$ROOT/typescript/plan"
  ["node_agent"]="$ROOT/typescript/node_agent"
  ["clustercontroller"]="$ROOT/typescript/clustercontroller"
)

generate_go() {
  local name="$1"
  local out="${GO_OUT[$name]}"
  mkdir -p "$out"
  echo "=> Generating Go stubs for $name"
  protoc "$PROTO_DIR/$name.proto" \
    -I "$PROTO_DIR" \
    --go_out=paths=source_relative:"$out" \
    --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:"$out"
}

generate_ts() {
  local name="$1"
  local out="${TS_OUT[$name]}"
  mkdir -p "$out"
  echo "=> Generating TypeScript grpc-web stubs for $name"
  protoc "$PROTO_DIR/$name.proto" \
    -I "$PROTO_DIR" \
    --js_out=import_style=commonjs:"$out" \
    --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:"$out"
}

if [[ "$GEN_GO" == "1" ]]; then
  for proto in "${!GO_OUT[@]}"; do
    generate_go "$proto"
  done
fi

if [[ "$GEN_TS" == "1" ]]; then
  if ! command -v protoc-gen-grpc-web >/dev/null 2>&1; then
    die "missing required plugin: protoc-gen-grpc-web (set GEN_TS=0 to skip TS generation)"
  fi
  for proto in "${!TS_OUT[@]}"; do
    generate_ts "$proto"
  done
fi

if [[ "$GEN_PY" == "1" ]]; then
  die "Python generation not configured; set GEN_PY=0 or extend this script."
fi

echo "Proto generation complete. Run 'git diff --stat' to review changes."
