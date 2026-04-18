# Bootstrap Security Contract

Bootstrap mode is a temporary, tightly-restricted authorization bypass used exclusively during cluster Day-0 initialization. It is not a standing access path. It exists for one purpose: allow the cluster to be wired up before normal RBAC bindings exist.

---

## Who Can Use It

Bootstrap mode gates access by **source address** and **caller identity**, not by role:

**Interceptor-level (loopback gate):** Requests arriving from `127.0.0.1` or `::1` on an allowlisted gRPC method are marked `IsBootstrap: true`. These requests bypass RBAC entirely because the interceptor has already verified they came from the local machine within the time window.

**Handler-level (authenticated SA gate):** Requests from non-loopback addresses (e.g., the CLI connecting through Envoy) are allowed if:
1. The caller is authenticated (valid JWT or mTLS)
2. The caller's subject is one of the explicit bootstrap service accounts: `globular-node-agent`, `globular-controller`, or `globular-gateway`
3. The bootstrap flag file is active and unexpired

No other caller identity gets bootstrap access. Anonymous requests, user accounts, and service accounts not on that list are rejected through the normal RBAC path even while the flag file is present.

---

## When It Exists

Bootstrap mode is active when **all three conditions hold simultaneously**:

1. The flag file exists at `/var/lib/globular/bootstrap.enabled`
2. The file contains valid JSON with `enabled_at_unix` and `expires_at_unix` fields
3. The current time is between `enabled_at_unix` and `expires_at_unix`

The flag file must:
- Have mode `0600` (owner read/write only)
- Be owned by `root` (uid 0) or the `globular` service user
- Contain a JSON object with valid timestamps — plain text, wrong permissions, or malformed JSON are rejected silently (the gate closes, not crashes)

---

## How It Expires

Bootstrap mode expires in two ways:

**Automatic (time-based):** The `expires_at_unix` timestamp is checked on every request. Once the current time exceeds it, the gate closes and the flag file is deleted automatically. The default TTL written by `EnableBootstrapGate` is 30 minutes.

**Manual:** Delete the flag file:
```bash
rm /var/lib/globular/bootstrap.enabled
```
Or use the CLI:
```bash
globular cluster bootstrap --disable
```

The file is also cleaned up automatically when `isWithinTimeWindow` detects expiry — so even if no request comes in, a process restart will find the gate closed.

---

## What Denies Access

Even while the flag file is valid, a bootstrap request is denied if:

| Check | Denial reason |
|---|---|
| Flag file missing or stat fails | `bootstrap_not_enabled` — fast path, no log |
| File is corrupt, partial JSON, or timestamps invalid | `bootstrap_expired` |
| `expires_at_unix` ≤ current time | `bootstrap_expired` |
| `enabled_at_unix` is in the future (clock skew attack) | `bootstrap_expired` |
| File permissions are not `0600` | `bootstrap_expired` |
| File is not owned by root or globular user | `bootstrap_expired` |
| Request source is not loopback (interceptor path) | `bootstrap_remote` |
| gRPC method not in the explicit allowlist (interceptor path) | `bootstrap_method_blocked` |
| Caller subject not in bootstrap SA allowlist (handler path 2) | falls through to normal RBAC check |

Every denial from gates 2–4 is logged at `WARN` with the subject, method, and reason. The allow is logged at `INFO`.

---

## What Bootstrap Does NOT Grant

Bootstrap mode is deliberately narrow:

- It does **not** bypass TLS — all connections still require the cluster CA certificate
- It does **not** bypass authentication — handler-level path 2 requires a valid token
- It does **not** grant access to arbitrary methods — only the [explicit allowlist](../../golang/security/bootstrap.go) (health, RBAC, auth, resource identity, DNS, repository, event)
- It does **not** persist — once the flag expires or is removed, every subsequent request goes through normal RBAC with no exceptions
- It is **not** a recovery mechanism — if RBAC bindings are lost after bootstrap, recover using the [node repair workflow](node-recovery.md), not by re-enabling bootstrap

---

## Bootstrap Is Not a Steady-State Auth Path

Once `globular rbac seed` completes and role bindings are in place, **no service or user should depend on bootstrap mode**. The RBAC service tests (`TestPostBootstrap_NormalRBACWorks`) verify that subjects with seeded bindings succeed through RBAC without the bootstrap flag present.

If you find yourself needing to re-enable bootstrap mode on a running cluster, that is a signal that role bindings are incomplete or missing — fix the bindings, don't extend the bootstrap window.

---

## See Also

- [Access Control: Roles and Permissions](rbac-permissions.md) — Assigning roles, CLI usage, Day-0 seeding
- [Day-0 / Day-1 / Day-2 Operations](day-0-1-2-operations.md) — Full cluster lifecycle including bootstrap sequence
- [Security Architecture](security.md) — Complete interceptor chain, authentication, PKI
