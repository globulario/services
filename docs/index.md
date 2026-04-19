# Globular Documentation

> An open-source microservices platform for self-hosted distributed applications

Globular runs native Linux binaries under systemd, orchestrated by workflows, with etcd as the single source of truth — no containers, no Kubernetes, no cloud provider required.

---

## Get Started

New to Globular? Start here.

- [Getting Started](getting-started.md) — Go from zero to a running cluster in 15 minutes
- [MCP Setup](operators/mcp-setup.md) — Connect Claude Code to your cluster for AI-assisted management
- [What is Globular](operators/what-is-globular.md) — Platform overview, core components, and how it compares to Kubernetes
- [Architecture Overview](operators/architecture-overview.md) — How the control plane, data plane, and state management work together

---

## Concepts

Understand how Globular works before operating it.

- [Why Globular](concepts/why-globular.md) — Design philosophy: why workflows, why no containers, why etcd
- [Deployment Philosophy](operators/deployment-philosophy.md) — The escalator principle, why rollback is forbidden, graceful degradation
- [Glossary](concepts/glossary.md) — Key terms and definitions for the Globular platform
- [Convergence Model](operators/convergence-model.md) — How desired state becomes reality through 4 truth layers
- [Workflows](operators/workflows.md) — The execution engine: phases, failure classification, retry, audit
- [Services and Packages](operators/services-and-packages.md) — How services are structured, built, packaged, and published
- [Security](operators/security.md) — PKI, JWT/mTLS authentication, RBAC, bootstrap security
- [Access Control: Roles and Permissions](operators/rbac-permissions.md) — Built-in roles, assigning access, resource permissions, Day-0 seeding
- [Bootstrap Security Contract](operators/bootstrap-security.md) — Who can use it, when it exists, how it expires, what denies access

---

## Tasks

Step-by-step procedures for common operations. Each task is executable from start to finish.

- [Deploy an Application](tasks/deploy-application.md) — Publish a package and deploy it across the cluster
- [Publish a Service](tasks/publish-service.md) — Build, package, and publish a service to the repository
- [Add a Node](tasks/add-node.md) — Expand the cluster with a new machine
- [Update the Cluster](tasks/update-cluster.md) — Upgrade services and infrastructure
- [Debug a Failed Workflow](tasks/debug-failed-workflow.md) — Diagnose why a deployment or upgrade failed
- [Recover a Node](tasks/recover-node.md) — Restore a failed or unreachable node

---

## Cluster Operations

Day-0 through Day-2 operational guides.

- [Day-0 / Day-1 / Day-2 Operations](operators/day-0-1-2-operations.md) — Complete lifecycle from first boot to ongoing maintenance
- [Installation (Day-0)](operators/installation.md) — Bootstrap the first node
- [Adding Nodes (Day-1)](operators/adding-nodes.md) — Join tokens, approval, profile assignment
- [Deploying Applications](operators/deploying-applications.md) — Desired-state model, monitoring, repair
- [Repository Overview](operators/repository-overview.md) — Philosophy, identity model, state machine, GC, invariants
- [Publishing Services](operators/publishing-services.md) — Build and publish packages, artifact lifecycle
- [Updating the Cluster](operators/updating-the-cluster.md) — Service upgrades, infrastructure upgrades, rollback
- [Debugging Failures](operators/debugging-failures.md) — Workflow diagnostics, service logs, common patterns
- [Observability](operators/observability.md) — Prometheus metrics, log aggregation, workflow history, MCP tools
- [Backup and Restore](operators/backup-and-restore.md) — Providers, scheduling, retention, disaster recovery
- [Building from Source](operators/building-from-source.md) — Clone, build, and install from Git repositories
- [Ports Reference](operators/ports-reference.md) — All ports, firewall rules, network requirements
- [Repository Repair](operators/repository-repair.md) — Diagnose and repair repository, desired-state, and installed-state inconsistencies
- [Known Issues](operators/known-issues.md) — CLI gaps, infrastructure limitations, planned fixes

---

## Advanced Operations

High availability, failure handling, networking, and certificate management.

- [High Availability](operators/high-availability.md) — Leader election, etcd quorum, MinIO erasure coding, failover
- [Cluster Self-Healing Reference](operators/cluster-self-healing.md) — What the cluster fixes automatically, what it cannot, operator signals and actions
- [Failure Scenarios and Recovery](operators/failure-scenarios.md) — Infrastructure, service, and node failure catalog
- [Node Full-Reseed Recovery](operators/node-recovery.md) — Complete wipe-and-rebuild workflow: snapshot, fencing, reprovision, reseed, verify
- [Platform Status](operators/platform-status.md) — What is implemented, partial, planned, and intentionally unsupported today
- [Cluster Doctor](operators/cluster-doctor.md) — Invariant checking, auto-heal, remediation workflows
- [Network and Routing](operators/network-and-routing.md) — Envoy gateway, xDS, DNS, service discovery, gRPC-Web
- [Certificate Lifecycle](operators/certificate-lifecycle.md) — Provisioning, rotation, monitoring, troubleshooting
- [DNS and PKI](operators/dns-and-pki.md) — Internal/external certificates, Let's Encrypt wildcards, DNS zones, split-horizon
- [Keepalived and Ingress](operators/keepalived-and-ingress.md) — VIP failover, DMZ configuration, external network access
- [Computing](operators/computing.md) — Distributed batch jobs, parallelized workloads, GPU scheduling

---

## AI Layer

How AI operates within Globular — rules, services, agent model, and integration guides.

- [AI Overview](ai/ai-overview.md) — What the AI layer is, what it does, implementation status
- [AI Rules](ai/ai-rules.md) — The strict operational rules AI agents must follow
- [AI Agent Model](ai/ai-agent-model.md) — What agents can observe, recommend, and execute
- [AI Services](ai/ai-services.md) — AI Memory, Executor, Watcher, and Router
- [AI Operator Guide](ai/ai-operator-guide.md) — Monitor, trust, constrain, and debug AI behavior
- [AI Developer Guide](ai/ai-developer-guide.md) — Build AI-safe services and MCP tools
- [AI Diagnosis Walkthrough](ai/ai-diagnosis-walkthrough.md) — End-to-end example: observe, diagnose, act, verify, record
- [AI Patterns and Anti-Patterns](ai/ai-patterns-and-anti-patterns.md) — Good and bad AI integration patterns

---

## Developers

Build services and applications for the Globular platform.

- [Local-First Development](developers/local-first.md) — Run services without a cluster, progressive deployment from laptop to production
- [Writing a Microservice](developers/writing-a-microservice.md) — Proto contract, code generation, server implementation, shared primitives
- [Service Packaging](developers/service-packaging.md) — Package format, spec files, build process
- [Publishing to Repository](developers/publishing-to-repository.md) — Publish workflow, provenance, CI/CD integration
- [RBAC Integration (quick reference)](developers/rbac-integration.md) — Proto annotations, permission model, role design
- [RBAC: Roles and Permissions (deep dive)](developers/rbac-roles-and-permissions.md) — Full model, worked example, patterns, testing, extension
- [Application Deployment](developers/application-deployment.md) — Web app packaging, gRPC-Web clients
- [Workflow Integration](developers/workflow-integration.md) — Health checks, backup hooks, graceful shutdown
- [Versioning](developers/versioning.md) — Semantic versioning policy, mono-version track, 1.0.0 criteria
