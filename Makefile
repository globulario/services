.PHONY: check-controller-no-exec check-nodeagent-exec-boundary check-target-paths-exist check-proto-authz check-no-misplaced-pb check-no-tracked-binaries gen-package-kinds check-package-kinds check-package-authority check-package-policy check-day0-package-contract check-services test-invariants test-integration test-integration-local test-integration-reconcile test-integration-release test-integration-migration test

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
	@# The production controller binary must NEVER launch processes (security
	@# boundary: no os/exec, syscall, or systemctl). This applies to shipped code
	@# only — *_test.go files are not compiled into the binary. Integration test
	@# harnesses (e.g. etcd_learner_harness_test.go) legitimately spawn a real etcd
	@# to prove FSM behaviour against etcd 3.5.14, and must live in `package main`
	@# to reach unexported symbols, so they are excluded here.
	@if grep -R --include='*.go' --exclude='*_test.go' -nE '"os/exec"|exec\.Command(Context)?\(' "$(CONTROLLER_DIR)"; then \
		echo "FAIL: forbidden exec usage found in $(CONTROLLER_DIR)"; \
		exit 1; \
	fi
	@echo "OK: no forbidden exec usage in cluster_controller_server (production code)"

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

# ── Package-kind single-source (registry.yaml) ───────────────────────────────
#
# packages/registry.yaml (sibling globulario/packages repo) is the SINGLE author of
# package kind. golang/packagekind/kinds_generated.go is a build-time projection that
# the node-agent (#7) and repository (#8) classifiers consume. Do NOT hand-edit the
# generated file or add another kind map — edit registry.yaml and run gen-package-kinds.
# See docs/design/package-classification-single-source.md.

gen-package-kinds:
	@cd golang && go run ./packagekind/cmd/genkinds

# Drift gate: committed kinds_generated.go must equal a fresh projection of
# registry.yaml. Skips (does not fail) when the packages repo isn't checked out, so
# the services-only CI still passes; the services-side TestComponentCatalogKindMatchesRegistry
# covers catalog drift without the packages repo, and this gate enforces registry
# parity wherever both repos are present.
check-package-kinds:
	@cd golang && tmp=$$(mktemp); go run ./packagekind/cmd/genkinds -out "$$tmp" 2>"$$tmp.err"; rc=$$?; \
	if [ $$rc -eq 2 ]; then \
		echo "SKIP: packages/registry.yaml not available (set GLOBULAR_PACKAGES_REGISTRY) — committed table used as-is"; \
		rm -f "$$tmp" "$$tmp.err"; \
	elif [ $$rc -ne 0 ]; then \
		echo "FAIL: genkinds gate (registry kind inconsistency — e.g. kind != form⊕role):"; \
		cat "$$tmp.err"; rm -f "$$tmp" "$$tmp.err"; exit 1; \
	elif diff -u packagekind/kinds_generated.go "$$tmp" >/dev/null; then \
		echo "OK: packagekind/kinds_generated.go matches registry.yaml (kind=form⊕role gate passed)"; \
		rm -f "$$tmp" "$$tmp.err"; \
	else \
		echo "FAIL: package-kind drift — kinds_generated.go is stale; run 'make gen-package-kinds':"; \
		diff -u packagekind/kinds_generated.go "$$tmp" || true; rm -f "$$tmp" "$$tmp.err"; exit 1; \
	fi

# ── Cross-repo package authority gate ───────────────────────────────────────
#
# packages/registry.yaml is the single authored package identity/index source.
# packages/metadata/<name>/specs/*.yaml is the canonical install-recipe source.
# globular-installer/internal/packagecatalog/specs/*.yaml is an embedded mirror.
# This gate fails on duplicate consumed spec roots, stale installer mirrors, or
# missing registry provenance for the embedded package catalog.

check-package-authority:
	@python3 scripts/validate-package-authority.py

# ── Packaged RBAC policy vocabulary ────────────────────────────────────────
#
# Invariant rbac.enforced_service_requires_packaged_policy_vocabulary: a
# gRPC-native service that participates in post-bootstrap RBAC enforcement must
# ship its generated permission/action vocabulary inside its package, so the
# node can register method->action mappings after the bootstrap gate closes.
# Scope is gRPC-native services ONLY — authzgen emits generated/policy/<svc>/
# only for services with (globular.auth.authz) RPCs; infra (scylla, envoy, etcd,
# keepalived, minio) and external packages have none and are never required to
# ship policy. CI-visible mirror of the in-build guard pkgpack.assertPackageGuards.
#
# Born from a real scar (v1.2.267): the package builder dropped the staged
# policy/, so every service package shipped no vocabulary and every role-based
# call was denied post-bootstrap. See docs/awareness/invariants.yaml.

check-package-policy:
	@python3 scripts/check-package-policy.py

# ── Day-0 package contract ─────────────────────────────────────────────────
#
# install-day0.sh must bootstrap every registry day0_required package and the
# transitive hard_deps required to make those packages usable on a fresh node.
# No unregistered package may remain in the bootstrap list.

check-day0-package-contract:
	@python3 scripts/validate-day0-package-contract.py

# ── Generated protobuf placement ─────────────────────────────────────────────
#
# check-no-misplaced-pb: protobuf-generated Go files must live ONLY under their
#   canonical package directory golang/<service>/<name>pb/. Root-level or
#   otherwise-misplaced *.pb.go files are invalid build artifacts — typically
#   debris from a protoc / generateCode.sh run launched from the wrong working
#   directory (they even compile under the wrong package path). This gate is
#   stronger than the .gitignore net: it catches already-tracked or force-added
#   debris, not just untracked files.
#
#   Born from a real scar: stale root-level cluster_controller{,_grpc}.pb.go
#   (older proto + older toolchain) sat next to the authoritative copies under
#   golang/cluster_controller/cluster_controllerpb/. See docs/awareness.

check-no-misplaced-pb:
	@echo "Checking for misplaced generated protobuf Go files..."
	@BAD=$$(find . -name '*.pb.go' \
	            | grep -vE '^\./golang/.*pb/[^/]*\.pb\.go$$' \
	            | grep -v '^\./vendor/' || true); \
	 if [ -n "$$BAD" ]; then \
		echo "FAIL: misplaced generated protobuf files (must live under golang/<service>/<name>pb/):"; \
		echo "$$BAD"; \
		echo "Delete these strays, or regenerate into the canonical package dir."; \
		exit 1; \
	 fi
	@echo "OK: all *.pb.go live under their canonical golang/<service>/<name>pb/ directory"

# ── No tracked binaries (general build-artifact gate) ────────────────────────
#
# check-no-tracked-binaries: the general form of check-no-misplaced-pb and the
#   /golang/*_server .gitignore rule. Fails if ANY git-tracked file is a compiled
#   binary (ELF / Mach-O / Windows PE / object / archive), detected by content
#   (libmagic), not by extension. Compiled binaries are build artifacts and never
#   belong in source control. Legit binary source (PNGs, fonts, fixtures) has
#   non-executable mime types and is not flagged.

check-no-tracked-binaries:
	@echo "Checking for tracked compiled binaries..."
	@bash scripts/check_no_tracked_binaries.sh

# ── Aggregate check target ───────────────────────────────────────────────────

check-services: check-controller-no-exec check-nodeagent-exec-boundary check-proto-authz check-no-misplaced-pb check-no-tracked-binaries check-package-kinds check-package-authority check-package-policy check-day0-package-contract

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
