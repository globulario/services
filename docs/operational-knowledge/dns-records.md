# Managing DNS records in Globular

Globular runs its own authoritative DNS service (`dns_server`, port 10006 gRPC, port 53 UDP/TCP). Public-facing zones like `globular.io` resolve through it — there is no external DNS provider, no Cloudflare console, no Route 53. This document is the canonical reference for adding, reading, and removing every supported record type, the gotchas the proto won't tell you, and the three paths an AI agent or operator can take when the CLI surface is incomplete.

If you just need the mail-setup procedure, jump to `runbooks/configure-google-workspace-mail.yaml`. This doc is the per-record reference.

---

## 1. Identity and entry points

| Layer | Endpoint | Used for |
|---|---|---|
| gRPC service | `dns.DnsService` (port 10006, mTLS) | Authoritative API. All other paths go through it. |
| Authoritative DNS | UDP/TCP 53 on every DNS-service node | Serves resolver queries. NS in the zone points here (`dns.globular.io`). |
| Public resolver path | 8.8.8.8 → root → `dns.globular.io` → answer | What npm/Gmail/the world sees. Use to verify propagation. |
| Per-package CLI | `globular dns {a,aaaa,txt,srv,inspect,lookup,...}` | Convenience wrapper. **Does not yet cover every record type.** |
| MCP tool | `mcp__globular__grpc_call dns.DnsService/Set*` | The most reliable path when the CLI is missing a subcommand or your local token is stale. |

**HARD RULE — never invent state.** DNS zones live in ScyllaDB. Re-reads after a restart hit the same store. If `dig` shows a record that the gRPC `Get*` doesn't, you have a propagation cache; if `Get*` shows it but `dig` doesn't, the resolver didn't reload. The gRPC store is authoritative.

---

## 2. Three ways to write a record

Pick the path that works in your current context. They all hit the same `dns.DnsService` server-side.

### 2.1 `globular dns ...` CLI — convenient, but **incomplete**

```
globular dns a set        <name> <ipv4> --ttl 3600
globular dns aaaa set     <name> <ipv6> --ttl 3600
globular dns txt set      <name> --value '<txt>' --ttl 3600
globular dns srv set      <name> --target <host> --port <p> --priority <pri> --weight <w> --ttl 3600
globular dns inspect      <name> --types A,AAAA,TXT,SRV
```

**The CLI is missing `mx`, `ns`, `cname`, `caa`, `soa`, `uri`, `afsdb`.** Proto and server fully support all of them — but no CLI subcommand exists yet. Don't waste time looking; use one of the other two paths until the CLI is closed.

The CLI uses the cached token at `~/.config/globular/token`. If the token is expired:

```
globular auth login --user sa
```

Tokens are JWT, ~15-minute TTL, auto-refreshed on use. Cache directory must be owned by the calling user (a stray `sudo globular` chowns it to root and silently breaks every subsequent call).

### 2.2 `grpc_call` via MCP — recommended for anything missing from the CLI

If you have access to the MCP toolset (`mcp__globular__grpc_call`), this is the path of least resistance: the MCP server holds its own auth context, so you don't need a fresh sa token in your shell.

```jsonc
// Set one MX
mcp__globular__grpc_call(
  service = "dns.DnsService",
  method  = "SetMx",
  request = '{"id":"globular.io","mx":{"preference":1,"mx":"ASPMX.L.GOOGLE.COM."},"ttl":3600}'
)

// Set TXT (REPLACES the values list — see §4)
mcp__globular__grpc_call(
  service = "dns.DnsService",
  method  = "SetText",
  request = '{"id":"globular.io","values":["v=spf1 include:_spf.google.com ~all"],"ttl":3600}'
)
```

Read paths (`GetA`, `GetMx`, `GetText`, etc.) are read-only and never need approval. Write paths require `read_only=false` in the MCP config — already set on this cluster.

### 2.3 `grpcurl` directly — fallback when MCP isn't around

```bash
TOKEN=$(cat ~/.config/globular/token)
grpcurl -insecure -cacert /var/lib/globular/pki/ca.pem \
  -H "token: $TOKEN" \
  -d '{"id":"globular.io","mx":{"preference":1,"mx":"ASPMX.L.GOOGLE.COM."},"ttl":3600}' \
  dns.globular.internal:10006 dns.DnsService/SetMx
```

The server accepts EITHER `token: <jwt>` metadata OR `authorization: Bearer <jwt>` — both forms work. Use `-H "token: $TOKEN"` to match how the rest of the Globular CLI sends it.

---

## 3. Record types — complete catalogue

Every Set/Get/Remove RPC operates against the same store keyed by `<TYPE>:<lowercased-name>`. The server normalizes names (lowercases, may add a trailing dot internally). You pass the name with or without a trailing dot — both work. **For target host fields**, the convention is: pass the FQDN with a trailing dot (`mail.example.com.`); the server will append one if you forget.

| Type | RPC family | Request field for name | Request payload | Add semantics | Notes |
|---|---|---|---|---|---|
| A (IPv4) | `SetA` / `GetA` / `RemoveA` | `domain` | `a: "10.0.0.63"` | **Append** to list of IPs for that name. Idempotent on duplicate. | Multiple A records on the same name = round-robin. |
| AAAA (IPv6) | `SetAAAA` / `GetAAAA` / `RemoveAAAA` | `domain` | `aaaa: "fe80::1"` | Append. | Same shape as A. |
| TXT (replace) | `SetText` / `GetText` / `RemoveText` | `id` | `values: ["v=spf1 ...", "..."]` | **REPLACE the entire list** with the array passed (dedup'd, trimmed). | Use this for multi-value updates where you want to be precise. |
| TXT (append) | `SetTXT` / `GetTXT` / `RemoveTXT` | `domain` | `txt: "v=spf1 ..."` | **APPEND** one value to the existing list (no-op on duplicate). Requires the domain to be in `GetDomains` (isManaged check). | Convenient for verification tokens. **Same storage key as SetText.** |
| MX | `SetMx` / `GetMx` / `RemoveMx` | `id` | `mx: {preference: N, mx: "host."}` | Append; **replaces** if the same `mx` host already present (matched on the FQDN). | Call once per MX target. |
| NS | `SetNs` / `GetNs` / `RemoveNs` | `id` | `ns: "dns.example.com."` | Append. | Multiple NS = secondary nameservers. |
| CNAME | `SetCName` / `GetCName` / `RemoveCName` | `id` | `cname: "target.example.com."` | Replaces (CNAMEs are single-valued by spec). | RFC: CNAME cannot coexist with other record types on the same name. |
| SRV | `SetSrv` / `GetSrv` / `RemoveSrv` | `id` (e.g. `_grpc._tcp.example.com`) | `srv: {priority, weight, port, target}` | Append. | Used by Globular's service discovery. |
| SOA | `SetSoa` / `GetSoa` / `RemoveSoa` | `id` | `soa: {ns, mbox, serial, refresh, retry, expire, minttl}` | Replace. | Usually written once at zone creation by `SetDomains`. |
| CAA | `SetCaa` / `GetCaa` / `RemoveCaa` | `id` | `caa: {flag, tag, domain}` | Append. | Controls which CAs may issue certs for the name. `tag` ∈ {issue, issuewild, iodef}. |
| URI | `SetUri` / `GetUri` / `RemoveUri` | `id` | `uri: {priority, weight, target}` | Append. | Niche; for service-pointer URIs. |
| AFSDB | `SetAfsdb` / `GetAfsdb` / `RemoveAfsdb` | `id` | `afsdb: {subtype, hostname}` | Append. | AFS database; rarely used outside research clusters. |
| Zone list | `SetDomains` / `GetDomains` | n/a | `domains: ["example.com", "..."]` | Replaces. | The set of zones this DNS is authoritative for. Required before `SetTXT`/some others will accept writes. |

Every Set RPC also takes `ttl: <seconds>` (uint32). If `0` (or omitted), the server applies a default — typically 300 seconds for TXT, longer for others. Specify it explicitly when you care.

### 3.1 The `id` vs `domain` field name inconsistency

The proto uses **`domain`** in `SetA`, `SetAAAA`, `SetTXT`. It uses **`id`** in `SetText`, `SetMx`, `SetCName`, `SetCaa`, `SetSrv`, `SetSoa`, `SetUri`, `SetNs`, `SetAfsdb`.

The value goes in the same place semantically (the name the record is for) — just under a different key. If you copy a JSON request template, double-check the field name against this table before submitting. The server returns `InvalidArgument` if you pick the wrong one and the rest of the request happens to validate.

### 3.2 The `SetText` vs `SetTXT` trap

Both write to the same storage key (`TXT:<lowercased-name>`) but differ in semantics:

| | `SetText` | `SetTXT` |
|---|---|---|
| Name field | `id` | `domain` |
| Value field | `values: []string` (full list) | `txt: string` (one) |
| Semantics | **Replaces** the entire list with the passed array (deduped, trimmed). | **Appends** the value; no-op if already present. |
| `isManaged` check | No. | Yes — domain must be in `GetDomains`. |
| Default TTL | Caller-supplied; no fallback. | 300 if omitted. |

If you alternate the two RPCs on the same name, expect surprises:

```
SetText(id="example.com", values=["a","b"])   # store: [a, b]
SetTXT (domain="example.com", txt="c")        # store: [a, b, c]
SetText(id="example.com", values=["d"])       # store: [d]   ← wipes a, b, c
```

**Pick one path per name and stick with it.** For multi-value TXT (SPF + Workspace verification combined), use `SetText` and pass the full set every call. That way the state is always exactly what's in the request — no accidental accumulation of stale verification tokens.

### Scope of the SetText trap — what it does and does NOT affect

The trap is per-name. Each DNS name has its own TXT storage key (`TXT:<lowercased-name>`), and the replacement only touches THAT key. Records on different names are completely independent:

| Record | DNS name | Storage key | Affected by a SetText on `<zone>`? |
|---|---|---|---|
| SPF | `<zone>` | `TXT:<zone>` | **YES** — IS the root TXT |
| Workspace domain verification | `<zone>` | `TXT:<zone>` | **YES** — shares root TXT with SPF |
| DKIM | `google._domainkey.<zone>` | `TXT:google._domainkey.<zone>` | NO — separate key |
| DMARC | `_dmarc.<zone>` | `TXT:_dmarc.<zone>` | NO — separate key |
| MX | `<zone>` | `MX:<zone>` | NO — different record type, different key |

So: when you publish DKIM or DMARC, SPF on the root is never at risk — you're writing a different storage key. The dangerous case is publishing the Workspace verification token, which lives on the same name as SPF. For that, either pass both values in one `SetText` call, or use `SetTXT` to append.

---

## 4. Reading records

Three valid paths, in increasing trust:

```bash
# 1) Authoritative storage (the gRPC store — the truth)
mcp__globular__grpc_call(service="dns.DnsService", method="GetMx",   request='{"id":"globular.io"}')
mcp__globular__grpc_call(service="dns.DnsService", method="GetText", request='{"id":"globular.io"}')
grpcurl -insecure -cacert /var/lib/globular/pki/ca.pem dns.globular.internal:10006 \
   dns.DnsService/GetMx -d '{"id":"globular.io"}'
globular dns inspect globular.io --types A,AAAA,TXT,SRV    # CLI; missing MX/NS/CNAME/CAA/etc.

# 2) Authoritative DNS query (what the DNS server serves over port 53)
dig +short MX  globular.io @dns.globular.io
dig +short TXT globular.io @dns.globular.io

# 3) Public resolver query (what the world sees, after propagation)
dig +short MX  globular.io @8.8.8.8
dig +short TXT globular.io @8.8.8.8
```

When triaging "the record isn't there yet", walk the three layers in order: gRPC → authoritative DNS → public resolver. If gRPC has it and authoritative DNS doesn't, the resolver in this process didn't pick up the new value (consider TTL/cache or a restart). If both authoritative and public agree, propagation is done.

---

## 5. Removing records

Every record type has a `Remove*` RPC. They take the same name field as the corresponding `Set*`, and a value identifier if multiple records can coexist (`a`, `aaaa`, `mx`, `ns`, `target`, etc.):

```jsonc
// Remove one MX target (keeps the others)
mcp__globular__grpc_call(
  service = "dns.DnsService",
  method  = "RemoveMx",
  request = '{"id":"globular.io","mx":"ALT4.ASPMX.L.GOOGLE.COM."}'
)

// Remove all TXT for a name (omit the txt field to clear all)
mcp__globular__grpc_call(
  service = "dns.DnsService",
  method  = "RemoveTXT",
  request = '{"domain":"globular.io"}'
)

// Remove one A record (keeps any other A records for the name)
mcp__globular__grpc_call(
  service = "dns.DnsService",
  method  = "RemoveA",
  request = '{"domain":"node-foo.globular.io","a":"10.0.0.99"}'
)
```

Read the proto comment for each Remove message — some accept an empty value to mean "remove all matching", others require you to be specific.

---

## 6. Worked examples — copy-paste ready

### 6.1 Set an A record (the most common task)

```jsonc
mcp__globular__grpc_call(
  service="dns.DnsService", method="SetA",
  request='{"domain":"api.globular.io","a":"10.0.0.63","ttl":300}')
```

Repeat with different IPs for round-robin. Verify:

```bash
dig +short A api.globular.io @dns.globular.io
dig +short A api.globular.io @8.8.8.8         # may take TTL seconds to propagate
```

### 6.2 Set Google Workspace MX (5 records) + SPF

The exact set we use on `globular.io`. Run six calls (five MX, one TXT):

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetMx",
  request='{"id":"globular.io","mx":{"preference":1, "mx":"ASPMX.L.GOOGLE.COM."},"ttl":3600}')
mcp__globular__grpc_call(service="dns.DnsService", method="SetMx",
  request='{"id":"globular.io","mx":{"preference":5, "mx":"ALT1.ASPMX.L.GOOGLE.COM."},"ttl":3600}')
mcp__globular__grpc_call(service="dns.DnsService", method="SetMx",
  request='{"id":"globular.io","mx":{"preference":5, "mx":"ALT2.ASPMX.L.GOOGLE.COM."},"ttl":3600}')
mcp__globular__grpc_call(service="dns.DnsService", method="SetMx",
  request='{"id":"globular.io","mx":{"preference":10,"mx":"ALT3.ASPMX.L.GOOGLE.COM."},"ttl":3600}')
mcp__globular__grpc_call(service="dns.DnsService", method="SetMx",
  request='{"id":"globular.io","mx":{"preference":10,"mx":"ALT4.ASPMX.L.GOOGLE.COM."},"ttl":3600}')

// SPF only — when DKIM/DMARC/verification land, REPLACE this list to include them all
mcp__globular__grpc_call(service="dns.DnsService", method="SetText",
  request='{"id":"globular.io","values":["v=spf1 include:_spf.google.com ~all"],"ttl":3600}')
```

Verify:

```bash
dig +short MX  globular.io @8.8.8.8
dig +short TXT globular.io @8.8.8.8
```

### 6.3 Add a Google Workspace verification token without wiping SPF

The wrong way (uses `SetText` with just the new value — wipes SPF):

```jsonc
// BAD — overwrites SPF
mcp__globular__grpc_call(service="dns.DnsService", method="SetText",
  request='{"id":"globular.io","values":["google-site-verification=abc123"],"ttl":3600}')
```

The right way (re-include the existing SPF entry in the same call):

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetText",
  request='{"id":"globular.io","values":["v=spf1 include:_spf.google.com ~all","google-site-verification=abc123"],"ttl":3600}')
```

Or, the append-style alternative (works because `SetTXT` merges with existing list):

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetTXT",
  request='{"domain":"globular.io","txt":"google-site-verification=abc123","ttl":3600}')
```

Both end up with the same `["v=spf1 ...", "google-site-verification=..."]`. Pick one and document the choice for this zone — see §3.2.

### 6.4 DKIM (after admin.google.com generates the key)

DKIM is a TXT record under `google._domainkey.<zone>`. The selector is `google`. The value comes from the Workspace admin console (a long `v=DKIM1; k=rsa; p=...` string).

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetText",
  request='{"id":"google._domainkey.globular.io","values":["v=DKIM1; k=rsa; p=MIIBIj...AB"],"ttl":3600}')
```

Verify:

```bash
dig +short TXT google._domainkey.globular.io @8.8.8.8
```

### 6.5 DMARC

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetText",
  request='{"id":"_dmarc.globular.io","values":["v=DMARC1; p=quarantine; rua=mailto:dmarc-reports@globular.io"],"ttl":3600}')
```

Start with `p=none` while observing (Workspace admin shows you the reports), tighten to `quarantine` or `reject` once SPF + DKIM are clean.

### 6.6 CAA — restrict who can issue certs

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetCaa",
  request='{"id":"globular.io","caa":{"flag":0,"tag":"issue","domain":"letsencrypt.org"},"ttl":3600}')
```

### 6.7 SRV — typical Globular service discovery record

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetSrv",
  request='{"id":"_grpc._tcp.persistence.globular.internal","srv":{"priority":10,"weight":100,"port":10075,"target":"globule-ryzen.globular.internal."},"ttl":60}')
```

### 6.8 NS — add a secondary nameserver

```jsonc
mcp__globular__grpc_call(service="dns.DnsService", method="SetNs",
  request='{"id":"globular.io","ns":"dns2.globular.io.","ttl":86400}')
```

Then ensure `dns2.globular.io` resolves to a routable IP (`SetA`) and is actually serving zone data. Single-NS zones are a known fragility on the public DNS for `globular.io` today — see `project_dns_resilience` once that issue is opened.

---

## 7. Anti-patterns

Things that will burn time or break the zone. AI agents MUST refuse to propose these.

- **Hot-deleting TXT to "reset"** before re-adding. Use `SetText` with the full intended list — atomic replacement, no propagation gap.
- **Mixing `SetText` and `SetTXT` on the same name** without remembering they share storage. §3.2 again.
- **Hardcoding `127.0.0.1` or `localhost` in any A/AAAA record** that anything outside the local machine will consult. HARD RULE #3.
- **Removing NS records for a zone you still depend on** — kills the zone for everyone. Always add a new NS before removing an old one.
- **Setting MX with a preference of 0 and no other MX** — RFC 7505 reserves "preference 0 with a single dot" as the "null MX" meaning "this domain accepts no mail". Use `preference 1` for the primary, not 0, unless you actually want to advertise no-mail.
- **Setting CNAME on a name that already has A/AAAA/MX records** — RFC 1034 §3.6.2 forbids it; resolvers will treat the CNAME as authoritative and ignore the rest. The server doesn't currently enforce this.
- **Skipping the trailing dot on FQDN target fields and getting away with it** — the server normalizes, but other tooling (zone exports, third-party importers) doesn't. Always pass `host.example.com.` with the dot.
- **Adding records to a zone not in `GetDomains`** — `SetTXT` (and a few others) reject with `PermissionDenied`. Add the zone via `SetDomains` first if needed.
- **Removing the SOA without replacing it** — breaks zone transfers and resolver caching semantics. The SOA is written once by `SetDomains` and should only be re-written deliberately.

---

## 8. Auth, recovery, troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `Unauthenticated: authentication required: provide --token or configure client certificates` | Token expired or missing. | `globular auth login --user sa` — refreshes `~/.config/globular/token`. Or use MCP `grpc_call` which holds its own context. |
| `PermissionDenied: the domain X is not managed by this DNS` | Zone not in `GetDomains`. | `SetDomains` first to include `X`. |
| `dig` returns NXDOMAIN but `Get*` returns the record | Resolver cache / TTL not expired / DNS service didn't reload after restart. | Wait the previous TTL out; `systemctl restart globular-dns.service` if persistent. |
| Records appear after restart with default config | Zones were re-registered from defaults; persisted zones in Scylla were missed. Known issue per CLAUDE.md — DNS zones can be in-memory. | Re-register via `SetDomains` + per-record set calls. |
| Single NS for the public zone | Architectural — `globular.io` only has `dns.globular.io.` listed as NS. | Add a second NS (`SetNs`) and ensure the secondary actually serves the zone. |
| Public dig works from internet but not from inside the cluster | Split-horizon DNS not supported; hairpin NAT issues. | Add the zone to `/etc/hosts` on the impacted nodes for now (known gap). |

Stale token symptoms:

- Token cache age: `stat -c '%y' ~/.config/globular/token`
- Decode payload: `cat ~/.config/globular/token | cut -d. -f2 | base64 -d` — look at `exp` (Unix epoch).
- If `~/.config/globular/` is owned by root from a prior `sudo`: `sudo chown -R $USER:$USER ~/.config/globular/`.

---

## 9. Known CLI gaps to close

These were identified while fixing the `globular.io` mail records on 2026-05-20 and are tracked here as the canonical inventory until the CLI catches up.

| Gap | Affected commands | Workaround |
|---|---|---|
| No `globular dns mx` subcommand | `mx get/set/remove` | `mcp__globular__grpc_call dns.DnsService/{Set,Get,Remove}Mx` |
| No `globular dns ns` subcommand | `ns get/set/remove` | Same via `grpc_call` |
| No `globular dns cname` subcommand | `cname get/set/remove` | Same |
| No `globular dns caa` subcommand | `caa get/set/remove` | Same |
| No `globular dns soa` subcommand | `soa get/set/remove` | Same |
| No `globular dns uri` / `afsdb` subcommands | rare types | Same |
| `globular dns inspect --types` only knows A,AAAA,TXT,SRV | inspecting MX, NS, etc. | Use `dig` against `dns.globular.io` + the appropriate `Get*` RPC. |

---

## 10. See also

- `runbooks/configure-google-workspace-mail.yaml` — phased procedure for putting `<zone>` on Google Workspace mail, including the verification token + DKIM + DMARC dance.
- `packages.md` — for how the DNS service itself is packaged and installed.
- `CLAUDE.md`, "KNOWN ISSUES" section — current operational gotchas around DNS persistence and split-horizon.
- `proto/dns.proto` — the authoritative API contract. When this document and the proto disagree, the proto wins.
