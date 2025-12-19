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
  "clustercontroller:golang/clustercontroller/clustercontrollerpb"
  "node_agent:golang/nodeagent/nodeagentpb"
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
  "clustercontroller"
  "node_agent"
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

echo "=> Building Go services"
bash "$REPO_ROOT/golang/build/build-services.sh"
