#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"
PROTO_DIR="$REPO_ROOT/proto"
GO_ROOT="$REPO_ROOT/golang"
TS_ROOT="$REPO_ROOT/typescript"

GO_TARGETS=(
  "resource:golang/resource/resourcepb"
  "rbac:golang/rbac/rbacpb"
  "log:golang/log/logpb"
  "dns:golang/dns/dnspb"
  "echo:golang/echo/echopb"
  "media:golang/media/mediapb"
  "search:golang/search/searchpb"
  "event:golang/event/eventpb"
  "storage:golang/storage/storagepb"
  "file:golang/file/filepb"
  "sql:golang/sql/sqlpb"
  "ldap:golang/ldap/ldappb"
  "mail:golang/mail/mailpb"
  "persistence:golang/persistence/persistencepb"
  "monitoring:golang/monitoring/monitoringpb"
  "spc:golang/spc/spcpb"
  "catalog:golang/catalog/catalogpb"
  "conversation:golang/conversation/conversationpb"
  "blog:golang/blog/blogpb"
  "authentication:golang/authentication/authenticationpb"
  "title:golang/title/titlepb"
  "torrent:golang/torrent/torrentpb"
  "discovery:golang/discovery/discoverypb"
  "repository:golang/repository/repositorypb"
  "cluster_controller:golang/cluster_controller/cluster_controllerpb"
  "node_agent:golang/node_agent/node_agentpb"
  "plan:golang/plan/planpb"
  "cluster_doctor:golang/cluster_doctor/cluster_doctorpb"
  "backup_manager:golang/backup_manager/backup_managerpb"
  "backup_hook:golang/backup_hook/backup_hookpb"
)

TS_TARGETS=(
  "authentication"
  "resource"
  "repository"
  "discovery"
  "echo"
  "media"
  "blog"
  "conversation"
  "search"
  "event"
  "storage"
  "file"
  "sql"
  "ldap"
  "mail"
  "persistence"
  "spc"
  "monitoring"
  "catalog"
  "log"
  "rbac"
  "title"
  "torrent"
  "dns"
"cluster_controller"
"node_agent"
"plan"
"cluster_doctor"
"backup_manager"
)

protoc_generate_go() {
  local proto="$1"
  local out="$2"
  local proto_path="$PROTO_DIR/$proto.proto"
  local go_out="$REPO_ROOT/$out"

  mkdir -p "$go_out"
  echo "=> Generating Go bindings for $proto"
  protoc "$PROTO_DIR/$proto.proto" \
    -I "$PROTO_DIR" \
    --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:"$go_out" \
    --go_out=paths=source_relative:"$go_out"
}

protoc_generate_ts() {
  local proto="$1"
  local out="$TS_ROOT/$proto"
  mkdir -p "$out"
  echo "=> Generating TypeScript grpc-web bindings for $proto"
  protoc --js_out=import_style=commonjs:"$out" -I "$PROTO_DIR" "$PROTO_DIR/$proto.proto"
  protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:"$out" -I "$PROTO_DIR" "$PROTO_DIR/$proto.proto"
}

for entry in "${GO_TARGETS[@]}"; do
  proto="${entry%%:*}"
  out="${entry#*:}"
  protoc_generate_go "$proto" "$out"
done

for proto in "${TS_TARGETS[@]}"; do
  protoc_generate_ts "$proto"
done

# Generate globular_auth_pb into a temp dir and copy into every TS service
# directory that imports it. The proto extends MethodOptions/FieldOptions and
# is imported by most service protos, so the generated .d.ts/.js files must
# exist alongside each service's generated code.
echo "=> Distributing globular_auth_pb to TypeScript service directories"
_auth_tmp="$(mktemp -d)"
protoc --js_out=import_style=commonjs:"$_auth_tmp" -I "$PROTO_DIR" "$PROTO_DIR/globular_auth.proto"
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:"$_auth_tmp" -I "$PROTO_DIR" "$PROTO_DIR/globular_auth.proto"
for proto in "${TS_TARGETS[@]}"; do
  svc_dir="$TS_ROOT/$proto"
  if [ -d "$svc_dir" ]; then
    cp -f "$_auth_tmp"/globular_auth_pb.* "$svc_dir/" 2>/dev/null || true
  fi
done
rm -rf "$_auth_tmp"

echo "=> Building Go services"
bash "$REPO_ROOT/golang/build/build-services.sh"

echo "=> Building globular CLI"
(
  cd "$GO_ROOT"
  GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go build -o globularcli/globularcli ./globularcli
)

echo "=> Building MCP server"
(
  cd "$GO_ROOT"
  GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go build -o tools/stage/linux-amd64/usr/local/bin/mcp ./mcp
)
# Also copy to packages/bin so build-all-packages.sh finds it
PACKAGES_BIN="$(cd "$REPO_ROOT/../packages/bin" 2>/dev/null && pwd)" || true
if [[ -d "$PACKAGES_BIN" ]]; then
  cp "$GO_ROOT/tools/stage/linux-amd64/usr/local/bin/mcp" "$PACKAGES_BIN/mcp"
  chmod +x "$PACKAGES_BIN/mcp"
  echo "   copied to $PACKAGES_BIN/mcp"
fi
