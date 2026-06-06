# Service Mobility

## Status

**Wired and runnable** as of 2026-06-06. The `globular service migrate`
CLI command exists, talks to the live cluster, validates planned
migrations end-to-end. Multi-node testing requires N≥2 and is the only
remaining gap to production use.

## What it is

Service mobility is the rebind half of recovery. When a Globular service
must move between nodes — for planned migration, node loss, capacity
rebalancing, or rolling upgrade — the recovery path is **not** a full
release-pipeline cycle on the new node. It is:

1. Start the binary on the target (it is already installed there via the
   normal release pipeline).
2. Wait for the target to register in etcd and pass its health probe.
3. Drain the source (let xDS routing pick up the new registration).
4. Stop the source.

For Scylla-backed services, the persistent state stays where it is —
shared between source and target during the overlap. For most cases this
collapses recovery from minutes (full reinstall) to seconds (rebind).

The principle is named in
[`meta.mobility_is_stronger_recovery_than_replication`](../awareness/state_authority_invariants.yaml)
and motivated by Eden Black 1985 §3.2.3, which observed that mobility is
strictly stronger than replication for the single-node-loss case
(replication preserves data but you still need to re-establish the
service somewhere; mobility does both together).

## What is wired up

`golang/services_mobility/` provides the orchestration package:

- `Orchestrator` — synchronous coordinator that walks one service from
  its current node to a target node.
- `NodeAgentController` interface — the surface needed from a node-agent
  client.
- `ServiceRegistry` interface — the etcd-backed view of "where is each
  service registered."
- `Outcome` — every migration produces a structured record naming each
  step taken, the source and target nodes, and the failure (if any).
  Append-only during orchestration; post-incident review reconstructs
  the timeline from the Outcome alone.

`golang/services_mobility/impl_etcd_registry.go` — production
`ServiceRegistry` that reads `/globular/services/<uuid>/config` and
matches by Name. Resolves the registered Address to a node-ID via the
caller-supplied IP→ID map.

`golang/services_mobility/impl_nodeagent.go` — production
`NodeAgentController` that holds a node-ID → AgentEndpoint map and
dials node-agents with the cluster's service certificate. Implements
all four interface methods (start, stop, binary-installed check,
reachability probe) against the generated protobuf client.

`golang/globularcli/service_migrate_cmd.go` + `service_migrate_etcd.go` —
the CLI command. Resolves the controller via etcd service registration,
calls ListNodes to build the topology, constructs the production impls,
runs the orchestrator. Supports `--dry-run`, `--json`, and tunables
for `--ready-timeout` / `--drain-grace` / `--poll-interval`.

## Usage

```
# Dry-run: validate the plan, no side effects.
globular service migrate ai-memory --to <node-id> --dry-run

# Real migration.
globular service migrate ai-memory --to <node-id>

# JSON output for scripting.
globular service migrate ai-memory --to <node-id> --json
```

When invoked from a node where the cluster's service certificates are
not at the default paths, set the GLOBULAR_PKI_CA, GLOBULAR_PKI_CERT,
and GLOBULAR_PKI_KEY environment variables to point at a valid
client cert.

## What is proven

The unit-test suite exercises every step with injected fakes,
including:

- The happy path with the full expected step sequence.
- Refusal to migrate a service that is not registered anywhere.
- No-op when the service is already on the target.
- Refusal of multi-instance services (out of scope for this prototype).
- Refusal when the target node is unreachable.
- Refusal when the binary is not installed on the target.
- Target failing to become healthy within the timeout — with cleanup of
  the half-started target.
- Stop-source failure — surfaces the error so operators see the
  two-instances-still-running state.
- Final-topology verification failure — catches race windows or etcd
  lag where the orchestrator's actions and the registry's view diverge.
- Context cancellation during the drain grace period.
- Outcome.FinishedAt propagates through named return (regression test
  for a Go defer-vs-return gotcha caught during live CLI testing —
  defer modifying a stack-local Outcome before `return out` does NOT
  propagate the FinishedAt stamp).

All eleven test cases pass. Additionally, the CLI was exercised against
the live cluster: `--dry-run` correctly resolved the registered service
and matched it against the requested target, identifying the no-op
case. `--json` output verified the named-return fix.

```
$ go test ./services_mobility/ -v
=== RUN   TestMigrate_HappyPath              --- PASS
=== RUN   TestMigrate_ServiceNotRunning      --- PASS
=== RUN   TestMigrate_AlreadyOnTarget        --- PASS
=== RUN   TestMigrate_MultiInstanceRejected  --- PASS
=== RUN   TestMigrate_TargetNotReachable     --- PASS
=== RUN   TestMigrate_BinaryNotInstalled     --- PASS
=== RUN   TestMigrate_TargetFailsToBecomeHealthy  --- PASS
=== RUN   TestMigrate_StopSourceFails        --- PASS
=== RUN   TestMigrate_VerifyFinalTopologyFails    --- PASS
=== RUN   TestMigrate_ContextCancelDuringGrace    --- PASS
PASS
ok  github.com/globulario/services/golang/services_mobility  0.140s
```

## What is NOT proven

The prototype is unit-tested with mocks. It has **not** been exercised
on a multi-node cluster. Specifically:

- The current production cluster is `N=1`, so the primitive cannot be
  end-to-end tested today. Mobility requires `N>=2` to demonstrate.
- The cutover window's behaviour under real xDS routing propagation
  latency is unverified. The default `DrainGracePeriod` of 10 seconds
  is a reasonable starting point but should be measured against the
  actual routing-update propagation time on your cluster.
- The behaviour under partial network partition between source and
  target during the overlap is unverified.
- In-flight RPC handover. Today, requests that hit the source at the
  moment systemd stops it will fail and need client-side retry. Full
  connection migration is a follow-up.

## What is NOT in scope yet

The CLI is wired but several capabilities are deliberate follow-ups:

- A proto definition for a `MigrateService` RPC on cluster_controller.
  Today the CLI orchestrates client-side; a server-side RPC would let
  programmatic callers (workflow service, ai-executor) invoke mobility
  without the CLI binary.
- A workflow YAML (`cluster.service.migrate`) wrapping the orchestrator
  with durable step receipts. Today an orchestrator-process crash mid-
  migration leaves the cluster in whatever step it reached; the
  workflow lift gives resumable mobility.
- An automatic mobility trigger based on cluster_controller's
  node-health watcher. Today mobility is operator-triggered.
- Multi-instance "rebalance" semantics (move K of N instances to
  spread load).
- A general framework for any Globular service. The current
  implementation assumes Scylla-backed state; other shapes (pure
  stateless, MinIO-backed, in-memory-heavy) will need variants of the
  same protocol.
- A proper health-probe step. Today `IsHealthy` is registration-presence
  proxy; a follow-up should call the service's own Health RPC.

## Path to production

Five concrete steps from prototype to production, in priority order:

1. **Workflow lift.** Wrap the orchestrator in a workflow YAML
   `cluster.service.migrate` so each step is durably recorded and
   partial migrations can resume after a controller restart. The
   actor for the workflow is the orchestrator code we have here;
   the workflow YAML names the steps and dispatches to actors.
2. **Proto + RPC + CLI.** Add `cluster_controllerpb.MigrateService`
   that dispatches the workflow; expose `globular service migrate
   <name> --to <node>`. This makes mobility operator-accessible.
3. **Real `NodeAgentController` implementation.** Wire the interface
   to the existing `node_agent_client` package. The `ControlService`
   RPC and the installed-package query already exist; this is glue.
4. **Real `ServiceRegistry` implementation.** Wire the interface to
   the existing etcd-backed service registry helpers (the same
   reads done by xDS and the cluster_controller's service view).
5. **Automatic mobility trigger.** cluster_controller's node-health
   watcher decides when to invoke mobility — node drain requested,
   capacity rebalance, planned upgrade window. This is where the
   `meta.bad_path_must_make_progress` principle pays out: instead of
   saturating the source under stress, move off.

Each step is independent and can ship in its own PR.

## How to think about it

Mobility is the architectural complement to a few other meta-principles
already in the graph:

- **`mobility_is_stronger_recovery_than_replication`** names the
  primitive. This prototype is its first implementation.
- **`MTTR_focus_outperforms_MTBF_for_evolving_systems`** explains why
  mobility is high-leverage — it is MTTR work at the architectural
  layer. Each migration that completes in seconds instead of minutes
  is N seconds of uptime saved.
- **`load_redirection_must_be_explicit_capacity_planning`** asks the
  successor question: when mobility moves a service, does the target
  have headroom? The orchestrator does not currently check this; a
  future enhancement should rank candidate targets by capacity before
  the operator picks.
- **`graceful_degradation_is_the_normal_mode_not_an_exception`** is
  what mobility enables. A saturated node can shed services to a
  healthier one without losing them entirely; the alternative
  (refuse new work, watch the node saturate) is what we have today.

Mobility is one of the architectural moves the awareness graph
predicted would be high-value. This prototype proves the move is
buildable and that its surface area is small. The remaining work is
glue and operational integration.
