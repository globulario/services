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
  "cluster_doctor:golang/cluster_doctor/cluster_doctorpb"
  "backup_manager:golang/backup_manager/backup_managerpb"
  "backup_hook:golang/backup_hook/backup_hookpb"
  "ai_memory:golang/ai_memory/ai_memorypb"
  "ai_watcher:golang/ai_watcher/ai_watcherpb"
  "ai_router:golang/ai_router/ai_routerpb"
  "ai_executor:golang/ai_executor/ai_executorpb"
  "workflow:golang/workflow/workflowpb"
  "compute:golang/compute/computepb"
  "compute_runner:golang/compute/compute_runnerpb"
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
"cluster_doctor"
"backup_manager"
"workflow"
"ai_executor"
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

# ── Vite CJS compatibility: clean globular_auth_pb ──────────────────────────
# protoc generates `require('./globular_auth_pb.js')` + `goog.object.extend(proto, ...)`
# in every _pb.js that imports globular_auth.proto. This breaks Vite: it chunks
# globular_auth_pb separately, creating split `proto` namespaces that make
# serializeBinary() fail with "Cannot read properties of undefined".
#
# The auth annotations are Go-side metadata only — TS code never uses them.
# Fix: strip the require/extend lines from all _pb.js, delete the globular_auth_pb
# files entirely, then sync cleaned source to dist/.
echo "=> Cleaning globular_auth_pb from TypeScript proto files (Vite compat)"
for proto in "${TS_TARGETS[@]}"; do
  svc_dir="$TS_ROOT/$proto"
  [ -d "$svc_dir" ] || continue
  # Remove the file itself — Vite chunks it even without explicit imports
  rm -f "$svc_dir"/globular_auth_pb.*
  # Strip require/extend lines from all generated _pb.js
  for pbjs in "$svc_dir"/*_pb.js; do
    [ -f "$pbjs" ] || continue
    if grep -q 'globular_auth_pb' "$pbjs"; then
      sed -i \
        -e '/var globular_auth_pb = require.*globular_auth_pb/d' \
        -e '/goog\.object\.extend(proto, globular_auth_pb)/d' \
        "$pbjs"
    fi
  done
  # Strip import lines from all generated _pb.d.ts (TypeScript declarations)
  for pbdts in "$svc_dir"/*_pb.d.ts; do
    [ -f "$pbdts" ] || continue
    if grep -q 'globular_auth_pb' "$pbdts"; then
      sed -i '/import.*globular_auth_pb/d' "$pbdts"
    fi
  done
  # Sync cleaned source to dist/
  if [ -d "$TS_ROOT/dist" ]; then
    mkdir -p "$TS_ROOT/dist/$proto"
    cp -f "$svc_dir"/* "$TS_ROOT/dist/$proto/" 2>/dev/null || true
  fi
done

# ── authzgen: extract permissions and roles from proto AuthzRule annotations ──
echo "=> Generating combined proto descriptor set"
ALL_PROTOS=()
for f in "$PROTO_DIR"/*.proto; do
  ALL_PROTOS+=("$f")
done
DESCRIPTOR_OUT="$REPO_ROOT/generated/policy/descriptor.pb"
mkdir -p "$(dirname "$DESCRIPTOR_OUT")"
protoc -I "$PROTO_DIR" --descriptor_set_out="$DESCRIPTOR_OUT" --include_imports "${ALL_PROTOS[@]}"

echo "=> Running authzgen to generate permissions and roles"
(
  cd "$GO_ROOT"
  GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go run ./globularcli/tools/authzgen \
    -descriptor "$DESCRIPTOR_OUT" \
    -out "$REPO_ROOT/generated/policy"
)
echo "=> authzgen complete"

echo "=> Updating globular-installer module dependency"
(
  cd "$GO_ROOT"
  GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go get github.com/globulario/globular-installer@latest 2>&1 || true
  go mod tidy 2>&1 || true
)

# Build the installer binary (used by Day-0 install script).
INSTALLER_ROOT="$(cd "$REPO_ROOT/../globular-installer" 2>/dev/null && pwd)" || true
if [[ -d "$INSTALLER_ROOT" ]]; then
  echo "=> Building globular-installer"
  (
    cd "$INSTALLER_ROOT"
    GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go build -o "$INSTALLER_ROOT/bin/globular-installer" ./cmd/installer 2>&1 || \
      echo "   WARN: installer build failed (may not have cmd/installer)"
  )
fi

# ── Sync workflow definitions into the service payload ────────────────────
echo "=> Syncing workflow definitions to payload"
WORKFLOW_DEFS="$GO_ROOT/workflow/definitions"
WORKFLOW_PAYLOAD="$REPO_ROOT/generated/payload/workflow/definitions"
if [[ -d "$WORKFLOW_DEFS" ]]; then
  mkdir -p "$WORKFLOW_PAYLOAD"
  cp "$WORKFLOW_DEFS"/*.yaml "$WORKFLOW_PAYLOAD/"
  echo "   copied $(ls "$WORKFLOW_PAYLOAD"/*.yaml | wc -l) definitions"
fi

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

STAGE_BIN="$GO_ROOT/tools/stage/linux-amd64/usr/local/bin"
PACKAGES_BIN="$(cd "$REPO_ROOT/../packages/bin" 2>/dev/null && pwd)" || true
GLOBULAR_ROOT="$(cd "$REPO_ROOT/../Globular" 2>/dev/null && pwd)" || true

# Build gateway and xds from the Globular repo (different Go module).
# Output goes to the same stage directory as other services.
if [[ -d "$GLOBULAR_ROOT" ]]; then
  echo "=> Building gateway (from Globular repo)"
  (
    cd "$GLOBULAR_ROOT"
    GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go build -o "$STAGE_BIN/gateway" ./cmd/gateway
  )
  echo "=> Building xds (from Globular repo)"
  (
    cd "$GLOBULAR_ROOT"
    GOCACHE="${GOCACHE:-/tmp/.cache/go-build}" go build -o "$STAGE_BIN/xds" ./cmd/xds
  )
else
  echo "=> WARN: Globular repo not found at $REPO_ROOT/../Globular — skipping gateway/xds build"
fi

# Copy binaries to packages/bin so build-all-packages.sh finds them
if [[ -d "$PACKAGES_BIN" ]]; then
  for bin in mcp gateway xds; do
    if [[ -f "$STAGE_BIN/$bin" ]]; then
      cp "$STAGE_BIN/$bin" "$PACKAGES_BIN/$bin"
      chmod +x "$PACKAGES_BIN/$bin"
      echo "   copied $bin to $PACKAGES_BIN/"
    fi
  done
fi
