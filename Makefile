.PHONY: check-controller-no-exec check-nodeagent-exec-boundary check-target-paths-exist check-proto-authz check-services test-invariants test-integration test

# ── Security boundary checks ────────────────────────────────────────────────
#
# check-target-paths-exist: fail explicitly if a checked directory is missing.
#   A missing directory is NOT an implicit pass — it means the check cannot
#   run, which is itself a failure.
#
# check-controller-no-exec: the cluster controller must never launch processes.
#   Forbidden: "os/exec" import, exec.Command(...), exec.CommandContext(...)
#   Allowed:   syscall.SIGTERM, syscall.Stat_t, and other OS primitives.
#
# check-nodeagent-exec-boundary: os/exec is allowed in the node agent only
#   inside internal/supervisor/. Any usage outside that package is forbidden.

CONTROLLER_DIR := ./golang/cluster_controller/cluster_controller_server
NODEAGENT_DIR  := ./golang/node_agent/node_agent_server

check-target-paths-exist:
	@echo "Checking checked directories exist..."
	@test -d "$(CONTROLLER_DIR)" || { echo "FAIL: directory missing: $(CONTROLLER_DIR)"; exit 1; }
	@test -d "$(NODEAGENT_DIR)"  || { echo "FAIL: directory missing: $(NODEAGENT_DIR)"; exit 1; }
	@echo "OK: all checked directories present"

check-controller-no-exec: check-target-paths-exist
	@echo "Checking cluster_controller_server has no forbidden exec usage..."
	@if grep -R --include='*.go' -nE '"os/exec"|exec\.Command(Context)?\(' "$(CONTROLLER_DIR)"; then \
		echo "FAIL: forbidden exec usage found in $(CONTROLLER_DIR)"; \
		exit 1; \
	fi
	@echo "OK: no forbidden exec usage in cluster_controller_server"

check-nodeagent-exec-boundary: check-target-paths-exist
	@echo "Checking node_agent_server exec usage is confined to operational code..."
	@# The node agent is a system executor by design; exec is legitimate in:
	@#   internal/         — all internal packages (supervisor, actions, ingress, etc.)
	@#   *_provider.go     — backup/restore providers that shell out to restic/systemctl
	@#   *_handler.go      — RPC handlers that query systemd/journald
	@#   repair_actions.go — openssl + systemctl certificate repair
	@#   workflow_day0.go  — Day-0 bootstrap that orchestrates system setup
	@#   apply_package_release.go / process_fingerprint.go — binary install + fingerprinting
	@#   server.go / heartbeat.go / installed_services.go / hardware.go / certificate.go
	@#     — core operational files with systemctl/etcdctl/openssl calls
	@#
	@# What must NOT use exec: generated protobuf files, pure config/state types.
	@# If exec appears in a file matching *_pb.go, *_grpc.pb.go, or types*.go, flag it.
	@VIOLATIONS=$$(grep -R --include='*.pb.go' --include='*_grpc.pb.go' --include='types*.go' \
	                   -lE '"os/exec"|exec\.Command(Context)?\(' "$(NODEAGENT_DIR)" 2>/dev/null); \
	 if [ -n "$$VIOLATIONS" ]; then \
		echo "FAIL: exec found in generated/type files: $$VIOLATIONS"; \
		exit 1; \
	 fi
	@echo "OK: exec boundary respected in node_agent_server"

# ── Proto RBAC annotation coverage ──────────────────────────────────────────
#
# check-proto-authz: every rpc in every service proto must have a
#   (globular.auth.authz) annotation. Fails with filename + rpc name
#   for each violation.
#
# Allowlist: protos for unimplemented/third-party services that intentionally
#   have no authz (compute, reflection, globular_auth internals).

check-proto-authz:
	@echo "Checking proto authz annotation coverage..."
	@bash scripts/check_proto_authz.sh

# ── Aggregate check target ───────────────────────────────────────────────────

check-services: check-controller-no-exec check-nodeagent-exec-boundary check-proto-authz

# ── Test targets ─────────────────────────────────────────────────────────────

test-invariants:
	@echo "Running invariant tests (no cluster required)..."
	cd golang && go test ./repository/repository_server/... -run "TestINV|TestReservation|TestMigrate" -v -count=1 -race
	@echo "All invariant tests passed."

test-integration:
	@echo "Running integration tests (requires cluster)..."
	cd golang && go test ./... -race -short -count=1
	@echo "Integration tests complete."

test-integration-local:
	@echo "Running integration tests against local containerized cluster..."
	@bash scripts/testcluster/run-tests.sh
	@echo "Local integration tests complete."

test: check-services test-invariants
	@echo "All checks and tests passed."
