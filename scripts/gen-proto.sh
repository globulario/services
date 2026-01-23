#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT/proto"

declare -A GO_OUT=(
  ["plan"]="$ROOT/golang/plan/planpb"
  ["node_agent"]="$ROOT/golang/nodeagent/nodeagentpb"
  ["clustercontroller"]="$ROOT/golang/clustercontroller/clustercontrollerpb"
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

for proto in "${!GO_OUT[@]}"; do
  generate_go "$proto"
done

echo "Proto generation complete. Run 'git diff --stat' to review changes."
