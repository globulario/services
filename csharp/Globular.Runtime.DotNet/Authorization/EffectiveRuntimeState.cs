// EffectiveRuntimeState — the resolved runtime configuration snapshot
// published to the cluster management plane so Globular can transparently
// inspect and manage the running service.
//
// This captures what the service ACTUALLY resolved at startup, not what
// was configured. It is the answer to "what is this service instance doing?"

using System.Text.Json.Serialization;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Complete effective runtime state for a Globular-managed service instance.
/// Published to etcd at registration and queryable via management endpoint.
/// </summary>
public sealed class EffectiveRuntimeState
{
    // ── Identity ────────────────────────────────────────────────────────

    [JsonPropertyName("service_name")]
    public string ServiceName { get; set; } = "";

    [JsonPropertyName("service_id")]
    public string ServiceId { get; set; } = "";

    [JsonPropertyName("version")]
    public string Version { get; set; } = "";

    [JsonPropertyName("publisher_id")]
    public string? PublisherId { get; set; }

    [JsonPropertyName("node_id")]
    public string? NodeId { get; set; }

    [JsonPropertyName("proto_package")]
    public string? ProtoPackage { get; set; }

    [JsonPropertyName("proto_service")]
    public string? ProtoService { get; set; }

    // ── Network ─────────────────────────────────────────────────────────

    [JsonPropertyName("grpc_address")]
    public string GrpcAddress { get; set; } = "";

    [JsonPropertyName("grpc_port")]
    public int GrpcPort { get; set; }

    [JsonPropertyName("domain")]
    public string? Domain { get; set; }

    [JsonPropertyName("protocol")]
    public string Protocol { get; set; } = "grpc";

    [JsonPropertyName("tls_enabled")]
    public bool TlsEnabled { get; set; }

    // ── Authorization ───────────────────────────────────────────────────

    [JsonPropertyName("authz_mode")]
    public string AuthzMode { get; set; } = "RbacStrict";

    [JsonPropertyName("rbac_active")]
    public bool RbacActive { get; set; }

    [JsonPropertyName("fallback_active")]
    public bool FallbackActive { get; set; }

    [JsonPropertyName("manifests_loaded")]
    public bool ManifestsLoaded { get; set; }

    [JsonPropertyName("permissions_source")]
    public string PermissionsSource { get; set; } = "none";

    [JsonPropertyName("manifest_schema_version")]
    public string? ManifestSchemaVersion { get; set; }

    [JsonPropertyName("permission_count")]
    public int PermissionCount { get; set; }

    [JsonPropertyName("action_mapping_count")]
    public int ActionMappingCount { get; set; }

    [JsonPropertyName("role_seed_status")]
    public string? RoleSeedStatus { get; set; }

    // ── Lifecycle ───────────────────────────────────────────────────────

    [JsonPropertyName("startup_time")]
    public DateTime StartupTime { get; set; } = DateTime.UtcNow;

    [JsonPropertyName("readiness")]
    public string Readiness { get; set; } = "starting";

    [JsonPropertyName("last_heartbeat")]
    public DateTime LastHeartbeat { get; set; } = DateTime.UtcNow;
}
