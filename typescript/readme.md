
# Globular Client Kit (lean, flexible, no service wrappers)

This mini-kit replaces brittle, hand-written GRPC wrappers and a rigid `services.ts` singleton
with a **small set of primitives** that are:
- **Extensible:** Instantiate any generated gRPC-web client class on the fly.
- **Composable:** One place to build metadata (auth token, domain, application).
- **HTTP-friendly:** Keep your few HTTP endpoints (uploads/downloads) separate from gRPC.
- **Framework-agnostic:** Works in plain web-components, React, Vue, etc.
- **Tauri-ready:** No assumptions about the host; just pass a custom `serviceLocator` when needed.

## Files

- `config.ts` — typed configuration (domain, protocol, ports, tokenProvider, application id) and a pluggable `serviceLocator`.
- `transport.ts` — helpers to build `grpc-web` metadata and instantiate generated clients at runtime.
- `http.ts` — **only** your HTTP utilities (upload, download, GET/POST JSON) with shared headers.
- `index.ts` — exports a tiny `globular` object to manage config and create clients; no baked-in service list.
- `examples/rbac.example.ts` — demonstrates direct use of generated `rbac` clients (no wrapper layer).

> Drop these files somewhere in your app (e.g. `apps/web/src/client/`) and import from there.
> The kit expects your generated code to be available (e.g. `@/gen/rbac_grpc_web_pb`, `@/gen/rbac_pb`).

## Why this structure

- **No redundant "api.ts":** HTTP endpoints live in `http.ts`. gRPC calls use generated clients directly.
- **No rigid `services.ts`:** Instead, a configurable `serviceLocator` maps `serviceId` → endpoint.
  Add a new service? Just generate code and call `globular.client(...)` with its constructor.
- **Auth & headers in one place:** `buildMetadata()` injects `Authorization`, `domain`, `application`, etc.

## Quick start

```ts
// 1) configure once at app bootstrap (e.g. main.ts)
import { globular, setConfig } from "./client";

setConfig({
  protocol: "https",
  domain: "example.com",
  ports: { https: 443, http: 80 },
  application: "globular-admin",
  tokenProvider: async () => localStorage.getItem("access_token") || ""
});

// Optional: override where services live
globular.setServiceLocator((serviceId) => {
  // Example: envoy routes all services via subpaths
  return `${globular.config.protocol}://${globular.config.domain}/grpc/${serviceId}`;
});

// 2) Use any generated client anywhere
import { RbacServiceClient } from "@/gen/rbac_grpc_web_pb";
import * as rbac from "@/gen/rbac_pb";

const rbacClient = globular.client("rbac.RbacService", RbacServiceClient);
const rq = new rbac.ListAccountsRqst();
const stream = rbacClient.listAccounts(rq, await globular.metadata());
stream.on("data", (rsp) => console.log(rsp.toObject()));
stream.on("error", (e) => console.error(e));
```

### HTTP helpers

```ts
import { http } from "./client";

// GET JSON
const cfg = await http.getJSON<{ version: string }>("/config");

// Upload a file
await http.upload("/uploads", file, { folder: "/tmp", overwrite: true });
```

## Migrating away from the old singleton

- Delete the hand-maintained per-service client fields and wrapper methods.
- Keep only one app-wide configuration and the service locator.
- Replace calls to `services.getRbacClient()` with:
  ```ts
  const rbacClient = globular.client("rbac.RbacService", RbacServiceClient);
  ```

## Tauri

In Tauri, you can still use `fetch` and `grpc-web` (with appropriate adapters or via an HTTP/2 proxy).
If your services are local, set a different `serviceLocator` at runtime (e.g., `https://localhost:9443` per service).
