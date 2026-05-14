# Developer Quickstart

This page is the fastest path from "I have the repo" to "I have a gRPC service
running inside a real Globular cluster." It is deliberately short — each step
links to the full reference when you want depth.

> **Audience**: a developer who wants to build a service for the platform.
> If you are operating an existing cluster, start at
> [Getting Started](../getting-started.md) instead.

---

## Prerequisites

- Go 1.24+
- `protoc` and the standard Go gRPC plugins
- A Globular cluster you can publish to. If you do not have one,
  [Local-First Development](local-first.md) shows how to skip the cluster
  entirely for the first few iterations.

```bash
git clone https://github.com/globulario/services.git
cd services
go mod download
```

---

## The five-step loop

```
1. proto          → define the contract
2. generateCode   → produce Go + TypeScript stubs
3. server         → implement handlers
4. package        → wrap in a Globular spec
5. publish/deploy → propagate via the desired-state model
```

That is the whole loop. Steps 1–3 run on your laptop. Steps 4–5 hit the
cluster. You can run steps 1–3 forever without a cluster
([Local-First](local-first.md)) before crossing the line.

### Step 1 — Define the contract

Create `proto/<service>.proto` with one `service` block and an
`(globular.auth.authz)` option on every RPC. The annotation is what wires
RBAC in for you at runtime — no handler code needed.

A complete example with request/response messages and resource templates is in
[Writing a Microservice → Step 1](writing-a-microservice.md#step-1-define-the-proto-contract).

### Step 2 — Generate code

```bash
./generateCode.sh
```

Generates Go server/client interfaces, TypeScript gRPC-Web stubs, and the
RBAC permission descriptors extracted from your authz annotations.

### Step 3 — Implement the server

Use the `globular_service` primitives — they hand you TLS, the interceptor
chain (auth → RBAC → audit), Prometheus metrics, the gRPC health endpoint,
graceful shutdown, and port allocation:

```go
func main() {
    globular_service.HandleInformationalFlags("myservice", "0.0.1")
    serviceID, configPath := globular_service.ParsePositionalArgs()

    srv := &server{config: mustLoad(configPath, serviceID)}
    lm := globular_service.NewLifecycleManager(srv, srv.config.Port)
    lm.RegisterService(func(gs *grpc.Server) {
        myservicepb.RegisterMyServiceServer(gs, srv)
    })
    lm.Serve() // blocks
}
```

You write handlers. The interceptor chain handles auth/RBAC before each call
reaches you. Full walkthrough:
[Writing a Microservice → Step 3](writing-a-microservice.md#step-3-implement-the-server).

### Step 4 — Package

Create a one-screen YAML spec:

```yaml
name: myservice
version: 0.0.1
publisher: you@example.com
platform: linux_amd64
kind: SERVICE
profiles:
  - custom
priority: 60
dependencies:
  - etcd
  - authentication
  - rbac
```

Build the package:

```bash
globular pkg build \
  --spec specs/myservice.yaml \
  --root packages/payload/myservice/ \
  --version 0.0.1
# → globular-myservice-0.0.1-linux_amd64-1.tgz
```

Details, payload layout, and signing: [Service Packaging](service-packaging.md).

### Step 5 — Publish and deploy

```bash
globular pkg publish globular-myservice-0.0.1-linux_amd64-1.tgz
globular services desired set myservice 0.0.1
globular services desired list   # APPLYING → INSTALLED
```

The controller writes desired state to etcd; the workflow service orchestrates
FETCH → VERIFY → INSTALL → START → HEALTH_CHECK on each target node; the node
agent executes locally. You watch it converge.

If something goes wrong, see
[Debugging Failures](../operators/debugging-failures.md) and
[the "what usually breaks first" section](../getting-started.md#what-usually-breaks-first)
of Getting Started.

---

## Before you edit existing code

If you are editing the awareness system, controller, workflow service,
node agent, repository, xDS, runtime, or MCP code: run **awareness preflight**
first. It returns the invariants you are about to cross, the forbidden
patterns, and the required tests:

```bash
globular awareness preflight --task "<what you are about to do>" --format agent
```

Or via MCP: `awareness.preflight`.

Read [Awareness](../awareness/index.md) for the full model — that one
preflight call has saved many wasted hours.

---

## Where to go next

| Goal | Doc |
|------|-----|
| Develop without a cluster | [Local-First Development](local-first.md) |
| Full microservice walkthrough | [Writing a Microservice](writing-a-microservice.md) |
| Package format reference | [Service Packaging](service-packaging.md) |
| Publish workflow + provenance | [Publishing to Repository](publishing-to-repository.md) |
| RBAC annotations + model | [RBAC Integration](rbac-integration.md) |
| Build a distributed batch job | [Writing a Compute Job](writing-a-compute-job.md) |
| Web app + gRPC-Web client | [Application Deployment](application-deployment.md) |
| Health checks, backup hooks, shutdown | [Workflow Integration](workflow-integration.md) |
| Versioning policy | [Versioning](versioning.md) |
| Run a fake cluster in containers | [Docker Simulation](docker-simulation.md) |
