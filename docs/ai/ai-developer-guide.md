# AI Developer Guide

This guide explains how to build services and systems that AI agents can operate safely within Globular. It covers how to expose safe actions to AI, how to make systems readable by AI, how to design APIs that AI can use without causing harm, and how to avoid unsafe patterns.

## Principles

When building services that AI will interact with, follow these design principles:

1. **Make state observable**: If AI can't see the state, it can't reason about it
2. **Make actions typed**: If AI must construct free-form commands, it will make mistakes
3. **Make effects reversible**: If AI takes a wrong action, recovery should be possible
4. **Make outcomes verifiable**: If AI can't verify its action worked, it can't learn
5. **Make boundaries explicit**: If AI doesn't know what it's not allowed to do, it will try everything

## Making Your Service Observable by AI

### Expose Health Through gRPC Health Protocol

Every Globular service automatically registers the gRPC health check. AI uses this to determine if a service is running. No extra work is needed for basic health.

For richer health reporting, implement custom health logic in `StartService()`:

```go
func (s *server) StartService() error {
    // Connect to database
    db, err := connectDB(s.config.DatabaseEndpoint)
    if err != nil {
        return err // Health check will report NOT_SERVING
    }
    s.db = db

    // Start background health monitor
    go s.monitorDatabaseHealth()

    return nil
}
```

### Register in etcd for Discovery

AI discovers services through etcd. The lifecycle manager handles this automatically. Ensure your service has:
- A stable service name (matches the proto package name)
- Configuration stored in etcd (not local files or environment variables)
- Endpoint registration that updates when the service restarts on a different port

### Expose Structured Diagnostics

If your service has internal state that's relevant for diagnosis, expose it through a diagnostic RPC:

```protobuf
rpc GetDiagnostics(GetDiagnosticsRequest) returns (GetDiagnosticsResponse) {
    option (globular.auth.authz) = {
        action: "inventory.diagnostics.read"
        permission: "read"
        resource_template: "/inventory/diagnostics"
        default_role_hint: "viewer"
    };
}

message GetDiagnosticsResponse {
    int64 uptime_seconds = 1;
    int32 active_connections = 2;
    int64 total_requests = 3;
    int64 error_count = 4;
    map<string, string> subsystem_status = 5;  // e.g., {"database": "healthy", "cache": "warming"}
}
```

This gives AI agents a structured view of your service's health beyond the binary gRPC health check.

### Emit Structured Events

When significant state changes occur, publish events through the Event service:

```go
func (s *server) onAssetCreated(asset *Asset) {
    s.eventClient.Publish(ctx, &eventpb.PublishRequest{
        Topic: "inventory.asset.created",
        Data: map[string]string{
            "asset_id": asset.Id,
            "category": asset.Category,
            "node_id":  s.nodeID,
        },
    })
}

func (s *server) onDatabaseError(err error) {
    s.eventClient.Publish(ctx, &eventpb.PublishRequest{
        Topic: "inventory.error.database",
        Data: map[string]string{
            "error":   err.Error(),
            "node_id": s.nodeID,
        },
    })
}
```

The AI Watcher subscribes to these events and can create incidents when patterns indicate problems.

### Use Structured Logging

Log with structured fields that AI can parse:

```go
slog.Error("database connection failed",
    "component", "inventory",
    "operation", "connect",
    "endpoint", dbEndpoint,
    "error", err.Error(),
    "retry_count", retryCount,
)
```

AI agents search logs via `nodeagent_search_logs` with pattern matching. Structured fields make pattern matching reliable.

## Designing Safe Actions for AI

### Use Typed Actions, Not Free-Form Commands

When your service needs to expose actions that AI might trigger, define them as typed RPCs:

```protobuf
// GOOD: Typed action with bounded parameters
rpc RebuildIndex(RebuildIndexRequest) returns (RebuildIndexResponse) {
    option (globular.auth.authz) = {
        action: "inventory.index.rebuild"
        permission: "admin"
        resource_template: "/inventory/index"
        default_role_hint: "admin"
    };
}

message RebuildIndexRequest {
    bool full_rebuild = 1;      // true = drop and recreate; false = incremental
    int32 batch_size = 2;       // items per batch (bounded by server, not client)
}
```

```protobuf
// BAD: Free-form command execution
rpc ExecuteCommand(ExecuteCommandRequest) returns (ExecuteCommandResponse);
message ExecuteCommandRequest {
    string command = 1;  // AI could put anything here
}
```

### Make Actions Idempotent

AI may retry actions (due to network failures, workflow retries, or misdiagnosis). Actions must be safe to execute twice:

```go
func (s *server) RebuildIndex(ctx context.Context, req *RebuildIndexRequest) (*RebuildIndexResponse, error) {
    // Check if rebuild is already in progress
    if s.indexRebuildInProgress.Load() {
        return &RebuildIndexResponse{
            Status: "already_in_progress",
            StartedAt: s.indexRebuildStartedAt,
        }, nil
    }

    // Start rebuild (idempotent: second call returns same state)
    s.indexRebuildInProgress.Store(true)
    s.indexRebuildStartedAt = time.Now().Unix()
    go s.doRebuild(req.FullRebuild, req.BatchSize)

    return &RebuildIndexResponse{Status: "started"}, nil
}
```

### Bound Parameters Server-Side

Never trust AI-provided parameters. Validate and bound them:

```go
func (s *server) RebuildIndex(ctx context.Context, req *RebuildIndexRequest) (*RebuildIndexResponse, error) {
    batchSize := req.BatchSize
    if batchSize <= 0 || batchSize > 1000 {
        batchSize = 100 // Default to safe value
    }
    // ...
}
```

### Return Verifiable Results

Actions should return results that AI can use to verify the outcome:

```go
message RebuildIndexResponse {
    string status = 1;           // "started", "completed", "already_in_progress", "failed"
    int64 started_at = 2;
    int64 completed_at = 3;
    int32 items_indexed = 4;
    int32 errors = 5;
    string error_message = 6;    // Only set if status == "failed"
}
```

AI can then check: "I requested index rebuild, the response says status=completed with 5000 items indexed and 0 errors — the action succeeded."

## Implementing Backup Hooks for AI Awareness

If your service manages data, implement the `BackupHookService` so the backup system (and AI) knows what data you own:

```go
func (s *server) PrepareBackup(ctx context.Context, req *backuppb.PrepareBackupRequest) (*backuppb.PrepareBackupResponse, error) {
    return &backuppb.PrepareBackupResponse{
        Entries: []*backuppb.ServiceDataEntry{
            {
                Name:             "inventory-data",
                Path:             "/var/lib/globular/inventory/data/",
                DataClass:        backuppb.DataClass_AUTHORITATIVE,
                Scope:            "cluster",
                SizeBytes:        s.getDataSize(),
                BackupByDefault:  true,
                RestoreByDefault: true,
                RebuildSupported: false,
            },
        },
    }, nil
}
```

This tells AI: "The inventory service owns authoritative data at this path, it must be backed up, and it cannot be rebuilt from other sources." This information prevents AI from accidentally clearing data it thinks is a cache.

## Exposing Service Actions to MCP

If you want external AI agents (Claude Code) to interact with your service through MCP, add tool definitions to the MCP server.

### Register a Tool

In `golang/mcp/tools_inventory.go`:

```go
func registerInventoryTools(s *MCPServer) {
    s.RegisterTool(ToolDefinition{
        Name:        "inventory_list_assets",
        Description: "List inventory assets with optional category and location filters",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "category": map[string]interface{}{
                    "type": "string",
                    "description": "Filter by category (e.g., 'hardware', 'software')",
                },
                "location": map[string]interface{}{
                    "type": "string",
                    "description": "Filter by location",
                },
                "limit": map[string]interface{}{
                    "type": "number",
                    "description": "Maximum results (default 20, max 100)",
                },
            },
        },
        Handler: func(args map[string]interface{}) (interface{}, error) {
            // Call the inventory service via gRPC
            client := s.getInventoryClient()
            resp, err := client.ListAssets(ctx, &inventorypb.ListAssetsRequest{
                CategoryFilter: getString(args, "category"),
                LocationFilter: getString(args, "location"),
                Limit:          getInt32(args, "limit", 20),
            })
            if err != nil {
                return nil, err
            }
            return formatAssets(resp.Assets), nil
        },
    })
}
```

### Register in Tool Groups

In `golang/mcp/register.go`:

```go
if g.Inventory {
    registerInventoryTools(s)
}
```

And add to the MCP config:
```json
{
    "tool_groups": {
        "inventory": true
    }
}
```

### Tool Design Rules

When designing MCP tools:

1. **Read-only by default**: Diagnostic tools should never modify state
2. **Bounded output**: Set `max_result_count` to prevent response flooding
3. **Typed parameters**: Use JSON schema with explicit types, not free-form strings
4. **Error messages**: Return clear, structured errors — AI needs to understand what went wrong
5. **Audit logging**: All tool invocations are logged by the MCP audit system

## Designing for AI-Safe Workflows

### Register Workflow Actions

If your service participates in workflows, implement the `WorkflowActorService`:

```go
func (s *server) ExecuteAction(ctx context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
    switch req.Action {
    case "inventory.rebuild_index":
        return s.handleRebuildIndex(ctx, req.Inputs)
    case "inventory.validate_data":
        return s.handleValidateData(ctx, req.Inputs)
    default:
        return nil, status.Errorf(codes.InvalidArgument, "unknown action: %s", req.Action)
    }
}
```

Action handlers must be:
- **Idempotent**: Safe to replay after failure
- **Bounded**: Complete within a reasonable timeout
- **Observable**: Log progress and errors
- **Typed**: Accept structured inputs, return structured outputs

### Make Actions AI-Diagnosable

When an action fails, return structured error information that AI can analyze:

```go
func (s *server) handleRebuildIndex(ctx context.Context, inputs map[string]string) (*workflowpb.ExecuteActionResponse, error) {
    err := s.rebuildIndex()
    if err != nil {
        return &workflowpb.ExecuteActionResponse{
            Ok: false,
            Output: map[string]string{
                "error":        err.Error(),
                "error_class":  classifyError(err),  // "database", "disk", "timeout", "config"
                "items_before_failure": strconv.Itoa(s.lastIndexCount),
                "suggestion":   "Check database connectivity and disk space",
            },
        }, nil
    }

    return &workflowpb.ExecuteActionResponse{
        Ok: true,
        Output: map[string]string{
            "items_indexed": strconv.Itoa(s.currentIndexCount),
            "duration_ms":   strconv.Itoa(int(duration.Milliseconds())),
        },
    }, nil
}
```

The `error_class` field helps AI classify the failure without parsing error message strings.

## Anti-Patterns to Avoid

### Don't Use Environment Variables for Configuration

```go
// BAD: AI can't see or modify environment variables
host := os.Getenv("DATABASE_HOST")

// GOOD: AI can query etcd to see the config
host := config.ResolveServiceEndpoint("database")
```

### Don't Log Unstructured Messages

```go
// BAD: AI can't reliably parse this
log.Printf("Error: something went wrong with %s at %d", service, timestamp)

// GOOD: AI can search for component="inventory" and operation="connect"
slog.Error("connection failed",
    "component", "inventory",
    "operation", "connect",
    "target", endpoint,
    "error", err.Error(),
)
```

### Don't Accept Free-Form Actions

```go
// BAD: AI could send anything
func (s *server) Execute(cmd string) error {
    return exec.Command("sh", "-c", cmd).Run()
}

// GOOD: Fixed action set, typed parameters
func (s *server) ExecuteAction(action string, params map[string]string) error {
    switch action {
    case "rebuild_index":
        return s.rebuildIndex(params["batch_size"])
    default:
        return fmt.Errorf("unknown action: %s", action)
    }
}
```

### Don't Hide State in Local Files

```go
// BAD: AI can't see this, and it's not replicated
f, _ := os.Create("/tmp/inventory_state.json")
json.NewEncoder(f).Encode(state)

// GOOD: State in etcd, visible and replicated
etcdClient.Put(ctx, "/globular/services/inventory/state", stateJSON)
```

### Don't Provide Unbounded Operations

```go
// BAD: Could delete everything
func (s *server) DeleteAll(ctx context.Context, req *DeleteAllRequest) (*Empty, error) {
    return s.store.DeleteAll()
}

// GOOD: Bounded, requires specific ID, audited via RBAC
func (s *server) DeleteAsset(ctx context.Context, req *DeleteAssetRequest) (*DeleteAssetResponse, error) {
    if req.AssetId == "" {
        return nil, status.Error(codes.InvalidArgument, "asset_id required")
    }
    return s.store.Delete(req.AssetId)
}
```

## Checklist for AI-Safe Service Design

Before deploying a service that AI will interact with:

- [ ] All configuration comes from etcd, not environment variables
- [ ] Service registers in etcd on startup (automatic via lifecycle manager)
- [ ] gRPC health check reflects actual service health
- [ ] All RPCs have RBAC annotations
- [ ] Mutating operations are typed (not free-form)
- [ ] Mutating operations are idempotent
- [ ] Parameters are validated and bounded server-side
- [ ] Responses include verifiable results (status, counts, error details)
- [ ] Error responses include structured error classification
- [ ] Events are published for significant state changes
- [ ] Logging uses structured fields (slog)
- [ ] Backup hooks declare data ownership and class
- [ ] No unbounded delete/clear operations
- [ ] No shell command execution

## What's Next

- [AI Patterns and Anti-Patterns](ai-patterns-and-anti-patterns.md): Concrete examples of good and bad AI integration
- [AI Rules](ai-rules.md): The complete constraint specification
