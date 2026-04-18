# Access Control: Roles and Permissions

Globular uses Role-Based Access Control (RBAC) to decide who can do what inside the cluster. Every gRPC request — whether it comes from a human user, a CI pipeline, or an internal service — is checked before it reaches any handler. If the caller doesn't have permission, the request is rejected with a clear error before any data is touched.

This page explains the model, the built-in roles, and how to manage access using the CLI.

---

## The Mental Model

Three concepts cover everything:

**Subjects** — who is making the request. A subject is any identity the platform can authenticate:

| Subject Type | Examples | How it authenticates |
|---|---|---|
| User account | `alice`, `bob@example.com` | Username + password → JWT token |
| Service account | `globular-controller`, `globular-node-agent` | Ed25519 service token (5-min TTL) |
| Node identity | `node_abc123` | mTLS client certificate |
| Group | `backend-team` | Membership in an account group |
| Application | `ci-pipeline` | Ed25519 application token |

**Roles** — named bundles of actions a subject is allowed to perform. You assign roles to subjects. The platform ships with 22 built-in roles. You can add your own.

**Permissions** — what a role allows. A permission is either a broad wildcard (`cluster_controller.*` = everything in the control plane) or a precise action (`repository.artifact.read` = list artifacts, nothing else). When a request arrives, the platform resolves the action for that gRPC method and checks whether the caller's roles include it.

The relationship is:
```
subject  →  [role, role, ...]  →  [action, action, ...]
```

A subject can have multiple roles. Roles accumulate — there is no hierarchy between roles, just union of allowed actions.

---

## How a Request Gets Authorized

Every gRPC request goes through a 7-step interceptor chain before the handler runs:

```
1. Call depth check      — prevent infinite service-to-service loops
2. Authentication        — validate JWT token or mTLS certificate
3. Bootstrap gate        — allow setup operations during Day-0 only
4. Cluster ID check      — reject tokens from other clusters
5. Allowlist check       — skip RBAC for health checks and auth endpoints
6. RBAC enforcement      — check subject's roles against the required action
7. Audit log             — record every allow and deny decision
```

Step 6 is where roles and permissions are checked. The interceptor:
1. Looks up the gRPC method in the permission descriptor (e.g., `/cluster_controller.ClusterController/GetHealth` → action `cluster_controller.cluster.health`)
2. Looks up the caller's roles from the RBAC service (`subject` → `[role1, role2, ...]`)
3. Checks whether any of those roles grants the required action
4. If no role matches, returns `PermissionDenied`

**Deny wins.** If a subject has an explicit deny on a resource, it overrides any allow — even from a higher-level role. This lets you carve out narrow exceptions without restructuring your role design.

**Inheritance.** Resource paths are hierarchical. Permission on `/inventory/assets` covers all paths beneath it (e.g., `/inventory/assets/laptop-001`). Permission on `/inventory` covers everything under it. This means you can grant broad or narrow access with a single rule.

---

## Built-In Roles

### Human User Roles

These roles are designed for people who interact with the cluster.

| Role | What it can do |
|---|---|
| `globular-viewer` | Read-only access across everything: workflow status, cluster health, node list, artifact catalog, monitoring metrics, AI observations, doctor reports |
| `globular-operator` | Everything viewer can do, plus: approve/reject join requests, trigger backup runs, start remediation workflows, manage DNS records, retry/cancel workflows |
| `globular-admin` | Full cluster access. All actions in all services via `/*` wildcard |
| `globular-security-admin` | Manage RBAC roles and bindings, manage accounts and organizations in the resource service, manage authentication |
| `globular-repository-admin` | Full repository management: publish, delete, GC, namespace management, entrypoint checksums |
| `globular-repository-editor` | Publish and read artifacts; cannot delete or manage namespaces |
| `globular-ai-admin` | Full control over AI layer: executor jobs, memory, router, watcher, cluster health read |
| `globular-ai-operator` | Read AI observations, query memory, read router status. Cannot trigger jobs |
| `globular-monitoring-viewer` | Prometheus queries only. Cannot access anything else |
| `globular-backup-admin` | Full backup management: configure, run, validate, restore planning |
| `globular-publisher` | CI/CD role: upload artifacts, manage releases, read discovery service |
| `globular-breakglass-admin` | Emergency full access. Identical to `globular-admin`. Reserved for break-glass scenarios |

### Service Account Roles

These roles are assigned to internal services, not humans. They define the minimum access each service needs.

| Role | Assigned to | What it can do |
|---|---|---|
| `globular-controller-sa` | Cluster Controller | Read/write desired state, manage releases, orchestrate nodes, read repository |
| `globular-node-agent-sa` | Node Agent | Report node status, read assigned packages, apply plans, read monitoring |
| `globular-node-executor` | Node Agent (per-execution) | Execute package installs and service controls, scoped to the executing node |
| `globular-workflow-writer-sa` | Workflow Service | Write workflow run records to etcd |
| `globular-repository-publisher-sa` | CI pipeline service account | Upload artifacts, read manifests |
| `globular-bootstrap-sa` | Day-0 initialization | RBAC seeding, account creation, DNS setup — active only during the 30-minute bootstrap window |
| `globular-ai-watcher-sa` | AI Watcher | Read cluster events and incidents; write observations |
| `globular-ai-memory-sa` | AI Memory | Read/write knowledge records in the memory store |
| `globular-ai-router-sa` | AI Router | Route AI requests; read executor status |
| `globular-ai-executor-sa` | AI Executor | Run diagnosis workflows; read cluster state; store observations |

---

## Managing Access with the CLI

### Assign a role to a user

```bash
# Give alice read-only cluster access
globular rbac bind --subject alice --role globular-viewer

# Promote alice to operator
globular rbac bind --subject alice --role globular-operator

# alice now has both roles (union of permissions)
```

### Remove a role

```bash
globular rbac unbind --subject alice --role globular-viewer
```

### Check what roles a subject has

```bash
globular rbac list-bindings --subject alice
# SUBJECT   ROLES
# alice     globular-operator
```

### List all role bindings

```bash
globular rbac list-bindings
# SUBJECT                    ROLES
# alice                      globular-operator
# globular-controller        globular-controller-sa
# globular-node-agent        globular-node-agent-sa
# globular-gateway           globular-admin
# ci-pipeline                globular-publisher
```

### Grant access to a specific resource (not a role)

Role bindings grant action-level permissions globally. For resource-scoped access — granting a user permission on one specific file or asset, not the whole collection — use resource permissions:

```bash
# Allow bob to write a specific asset but nothing else in that service
globular rbac set-permission \
  --subject bob \
  --resource "/inventory/assets/laptop-001" \
  --permission write
```

Resource permissions layer on top of role-based permissions. If bob has no role that grants `inventory.asset.write`, but has an explicit resource permission on `/inventory/assets/laptop-001`, his write is allowed on that path only.

---

## Security Properties

**Every request is checked.** There are no endpoints that skip RBAC except health checks (`/grpc.health.v1.Health/Check`) and the authentication endpoint itself. Even internal service-to-service calls go through the chain.

**Deny wins over allow.** If a subject has an explicit deny on a resource, it overrides any allow — including roles that would grant access. Use this to carve out narrow exceptions.

**Tokens are cluster-scoped.** A JWT from cluster A cannot be used on cluster B. The `ClusterID` claim in every token is validated against the local cluster domain. Replaying tokens across clusters returns `Unauthenticated`.

**Bootstrap is time-bounded.** During cluster installation, a 30-minute bootstrap window allows setup operations from loopback only. Once the window expires (or the flag file is deleted), the cluster enforces full RBAC with no exceptions.

**All decisions are audited.** Every authorization decision — allow or deny — is logged with the subject, method, resource, and reason. Denied requests are logged at WARN and are never sampled. Check the RBAC service logs or the structured audit stream for forensic review.

**RBAC survives a restart.** If the RBAC service is temporarily unreachable (restart, network issue), each service falls back to its locally-cached role manifest. Requests from subjects with known roles continue to be served. Requests requiring a live RBAC lookup fail closed until the service recovers.

---

## Day-0: Seeding Initial Access

On a freshly bootstrapped cluster, no role bindings exist. The first thing you do after installation is seed them:

```bash
# Enable the 30-minute bootstrap window on all nodes
globular cluster bootstrap --node globular.internal:11000

# Seed built-in service account bindings and all 22 cluster roles
globular rbac seed

# Assign yourself an admin role
globular rbac bind --subject your-username --role globular-admin

# Disable bootstrap mode (flag expires automatically after 30 min,
# but you can remove it immediately)
globular cluster bootstrap --disable
```

After seeding, all service accounts have their roles and you can log in normally with your admin account.

---

## Custom Roles

If the built-in roles don't fit your organization, you can add custom roles by editing `/etc/globular/policy/rbac/cluster-roles.json`. Files in `/etc/globular/` take precedence over package defaults and survive upgrades.

```json
{
  "version": "2.0",
  "roles": {
    "my-readonly-ci": [
      "repository.artifact.list",
      "repository.artifact.read",
      "repository.artifact.search"
    ],
    "inventory-viewer": [
      "inventory.asset.read",
      "inventory.asset.list"
    ],
    "inventory-editor": [
      "inventory.asset.read",
      "inventory.asset.list",
      "inventory.asset.write",
      "inventory.asset.create"
    ]
  }
}
```

After saving:
```bash
# Apply: restart the RBAC service to reload
systemctl restart globular-rbac.service

# Or hot-reload if supported:
globular rbac seed --force
```

Then assign the role like any built-in:
```bash
globular rbac bind --subject ci-pipeline --role my-readonly-ci
```

---

## See Also

- [Security Architecture](security.md) — PKI, JWT, mTLS, bootstrap, audit logging
- [RBAC Integration (Developers)](../developers/rbac-roles-and-permissions.md) — How services define permissions via proto annotations
- [Day-0 / Day-1 / Day-2 Operations](day-0-1-2-operations.md) — Full cluster lifecycle
