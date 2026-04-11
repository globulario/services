# Publishing Workflow Definitions to MinIO

Purpose: get workflow YAMLs from the repo into the runtime source of truth (MinIO bucket) so the workflow service loads them in production.

Prereqs:
- `mc` (MinIO client) installed and an alias configured, e.g. `mc alias set minio https://minio.globular.internal <access> <secret> --api S3v4`
- Access to the bucket/prefix used for workflows (default in docs: `globular-config/workflows/`)

Steps:
1) From repo root, sync definitions to MinIO:
```bash
mc cp golang/workflow/definitions/*.yaml minio/globular-config/workflows/
```

2) Verify upload:
```bash
mc ls minio/globular-config/workflows/
```

3) (Optional) Force workflow service to refresh (if it caches definitions):
- Restart workflow service, or
- Trigger whatever cache-bust endpoint is provided (if any).

4) Validate via MCP:
```bash
# list workflows/tools that rely on definitions
# (adapt to the relevant MCP tool if one exists)
```

Notes:
- The git copies under `golang/workflow/definitions/` are the reference; MinIO is the runtime source of truth.
- Keep versions in Git; treat MinIO as a deploy artifact store, not an editing surface.
