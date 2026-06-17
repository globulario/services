# Credential relocation epic — hand-off note

**Status:** MinIO pilot steps 1–2 landed in `services`. Steps 3–5 (cross-repo +
live cutover) and the fan-out to other services remain.
**Origin:** AWG meta-principle re-audit, 2026-06-17 (8/8 credential findings
verified real). Hard rule §6 — *no token/credential storage in etcd values; use
file references*.

---

## Decisions (locked)

| # | Decision |
|---|---|
| D1 | Secrets live at `/var/lib/globular/secrets/<service>/<name>` (dir 0700, files 0600, owned by `globular`). |
| D2 | Distribution: extend **Day-0/join** (same channel as PKI) to provision the secret files on every node. |
| D3 | Bootstrap root password: **Day-0 provisions a random root password** into the secret file; `authenticate()` then fails closed on empty (removes the `adminadmin` default-credential window). |
| D4 | Migration is two-phase: file-first read + `json:"-"`, *then* etcd scrub. No flag-day. |
| D5 | Pilot on MinIO end-to-end, then fan out. |

## The model (follows the existing `CertFile`/`KeyFile` precedent)

For each leaked credential: keep only a **path** in the config (etcd-safe);
read the **value** from a node-local 0600 file; prefer the file, fall back to
the inline value during migration; the writer publishes the path; provisioning
fills the file; then flip to file-only and scrub etcd.

---

## MinIO pilot — remaining steps

### ✅ Done (services, committed `1dc22067`)
- `config.MinIOConfig.SecretKeyFile` field; `(de)serialized` in `LoadMinIOConfig`/`SaveMinIOConfig`.
- `config.resolveSecretKey(inline, file)` — file-first, falls back to inline (tested: `config/minio_secret_file_test.go`).
- `const config.MinioRootSecretKeyFile = "/var/lib/globular/secrets/minio/root_secret_key"`.
- Controller `publishMinioConfigLocked` (cluster_controller_server/server.go ~1235) publishes the path alongside the inline value (dual-write).

### Step 3 — provision the secret file on every node  *(Globular repo + installer)*
- The MinIO root secret is `controllerState.MinioCredentials.RootPassword` (controller-generated). Each node needs it at `config.MinioRootSecretKeyFile` (0600, `globular:globular`).
- **Day-0** (installer): write the file when the founding node generates `MinioCredentials`.
- **Day-1 join** (`Globular/internal/gateway/handlers/cluster/` + the join script): the joining node must receive the secret over the existing authenticated join channel (same path that delivers PKI/CA material) and write it to `MinioRootSecretKeyFile`. Do **not** put it in the join URL/args; deliver it in the join response body alongside the cert bundle.
- Create `/var/lib/globular/secrets/minio/` (0700) in both paths.
- Acceptance: every node has the 0600 file with the correct value; `LoadMinIOConfig()` returns the file's value (verify by temporarily clearing the inline etcd value on a scratch node).

### Step 4 — flip MinIO to file-only  *(services)*
- Once **all** nodes are provisioned (verify cluster-wide), tag the inline value out of the etcd write: change `SaveMinIOConfig`'s `stored.SecretKey` to be omitted (or stop the controller from setting `cfg.SecretKey`). Keep `resolveSecretKey` (now the file is the only source).
- Also relocate the second copy: `ObjectStoreDesiredState.SecretKey` written to `/globular/objectstore/config` (see `cluster_controller_server/objectstore_config.go` ~61) — same treatment (publish a path, node-agent/doctor read the file).
- Update readers that consume `ObjectStoreDesiredState.SecretKey` (node-agent renderer, cluster-doctor) to read the file.

### Step 5 — scrub + rotate  *(live cluster, Day-1 op)*
- One-time: remove `secret_key` from `/globular/cluster/minio/config` and `/globular/objectstore/config` (overwrite via the controller after step 4 ships).
- Rotate the MinIO root credential (the old value was in etcd replicas + any plaintext JSON backups from `config/etcd_backup.go`).
- Also address `config/etcd_backup.go:71`: the whole-keyspace JSON backup must redact/omit credential keys (or encrypt) so the secret doesn't re-enter a plaintext artifact.

---

## Fan-out (after the pilot proves out)

Same 5-step pattern per service. The shared mechanism for the `srv.Save()` leakers
is: tag the credential field `json:"-"` (so `Utility.ToMap`→`SaveServiceConfiguration`
stops serializing it), add a `<Name>SecretFile` path field, load the value from
the file at startup, provision via Day-0/join.

| Service | Field(s) | Loader site |
|---|---|---|
| `resource/resource_server/server.go:864` | `Backend_user`/`Backend_password` (stop hardcoding `sa`/`adminadmin`) | server init |
| `mail/mail_server/config.go:50`, `handlers.go:462` | `Password`, connection `Password` | `StartImap`/`StartSmtp` |
| `catalog/catalog_server/handlers.go:62` | `Services[...]["Connections"][id]["Password"]` | `Save`/load |
| `sql/sql_server/sql.go:61` | connection `Password` | `CreateConnection`/load |
| `ai_executor/ai_executor_server/anthropic_client.go:301` | OAuth access+refresh token (etcd `/globular/secrets/...`) | `saveCredentials`/load |

## D3 — root-password bootstrap (rides with the auth fan-out)
- Day-0: generate a random root password, bcrypt-hash it into the credential, write the secret file.
- `authentication/.../handlers.go` `authenticate()`: remove the empty→`adminadmin` fallback and fail closed on empty. **Update the test** `TestAuthenticateUpgradesDefaultPasswordToBcrypt` (it currently encodes the old bootstrap behavior).

## Verification / rollback
- Each step 1–4 is independently revertable; only step 5 (scrub+rotate) is one-way (mitigated by rotation).
- Regression bar per service: a hermetic `resolve<Secret>` test (file-first + fallback), plus a build/`go vet`. End-to-end provisioning is validated on the cluster (no hermetic seam).

## Pointers
- Full design rationale: this note supersedes `/tmp/tier3-credential-relocation-design.md`.
- Audit findings detail: the AWG re-audit report (verified credential cluster).
