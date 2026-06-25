.PHONY: check-controller-no-exec check-nodeagent-exec-boundary check-target-paths-exist check-proto-authz check-services test-invariants test-integration test-integration-local test-integration-reconcile test-integration-release test-integration-migration test

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
	@# The node agent is a system executor by design, so exec is legitimate for
	@# read-only probes (systemctl is-active/status/show, journalctl, nodetool
	@# status, …) and domain tools (restic, sctool, cqlsh, mc, openssl) across its
	@# operational files. Two boundaries are enforced below:
	@#   1. exec must not appear in generated/type files (*_pb.go, types*.go).
	@#   2. MUTATING systemd UNIT actions (start/stop/restart/enable/disable/
	@#      daemon-reload/kill/mask/unmask) must go through internal/supervisor —
	@#      the single allowlisted, auditable systemd-control path (EX-2). The one
	@#      sanctioned exception is workflow_day0.go (Day-0 bootstrap runs before the
	@#      supervisor/etcd exist).
	@# If exec appears in a file matching *_pb.go, *_grpc.pb.go, or types*.go, flag it.
	@VIOLATIONS=$$(grep -R --include='*.pb.go' --include='*_grpc.pb.go' --include='types*.go' \
	                   -lE '"os/exec"|exec\.Command(Context)?\(' "$(NODEAGENT_DIR)" 2>/dev/null); \
	 if [ -n "$$VIOLATIONS" ]; then \
		echo "FAIL: exec found in generated/type files: $$VIOLATIONS"; \
		exit 1; \
	 fi
	@# Mutating systemd UNIT actions must go through internal/supervisor — the single
	@# allowlisted, auditable systemd-control path (EX-2 unit-control boundary). A
	@# direct exec.Command(... "systemctl", "<mutating-verb>" ...) anywhere else is
	@# forbidden. Read-only probes (is-active, status, show, list-units, …) are fine.
	@# workflow_day0.go is the one sanctioned exception: Day-0 bootstrap orchestrates
	@# system setup before the supervisor/etcd exist (bootstrap-boundary).
	@MUT=$$(grep -RnE 'exec\.Command(Context)?\([^)]*"systemctl"' --include='*.go' "$(NODEAGENT_DIR)" 2>/dev/null \
	          | grep -vE '/internal/supervisor/|/workflow_day0\.go:' \
	          | grep -iE '"(start|stop|restart|enable|disable|daemon-reload|kill|mask|unmask)"'); \
	 if [ -n "$$MUT" ]; then \
		echo "FAIL: mutating systemctl via raw exec outside internal/supervisor —"; \
		echo "      route these through the supervisor package (supervisor.Stop/Start/Restart/Enable/Disable/DaemonReload):"; \
		echo "$$MUT"; \
		exit 1; \
	 fi
	@echo "OK: exec boundary respected in node_agent_server (no mutating systemctl outside internal/supervisor)"

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

# ── Awareness graph checks ───────────────────────────────────────────────────
#
# The awareness validate/audit tooling was extracted out of this repo in
# commit 1c8a4888 ("extract awareness-graph from Globular: AWG becomes a
# standalone sidecar, not a cluster dependency") — the 11 `globularcli
# awareness *` command files were deleted. Run YAML validation / audit from
# the awareness-graph repo's own tooling against this repo's docs/awareness/
# YAML inputs. The former `make check-awareness*` targets pointed at the
# now-removed CLI command and could never pass; they were removed rather than
# re-coupling this repo to the extracted sidecar.

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

test-integration-reconcile:
	@echo "Running reconciliation scenario tests..."
	@bash scripts/testcluster/run-tests.sh reconcile
	@echo "Reconciliation tests complete."

test-integration-release:
	@echo "Running release pipeline tests..."
	@bash scripts/testcluster/run-tests.sh release
	@echo "Release tests complete."

test-integration-migration:
	@echo "Running ScyllaDB migration coordination tests..."
	@bash scripts/testcluster/run-tests.sh migration
	@echo "Migration tests complete."

test: check-services test-invariants
	@echo "All checks and tests passed."
