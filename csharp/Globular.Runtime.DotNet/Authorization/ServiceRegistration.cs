// ServiceRegistration — metadata published to etcd so Globular can inspect
// effective authorization/configuration state cluster-wide.
//
// This is not the authorization decision path — it's the observability path.
// Globular Admin, CLI tools, and cluster management use this to understand
// what each service instance is doing at runtime.

using System.Text.Json.Serialization;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Authorization and configuration metadata published to etcd as part of
/// service registration. Enables Globular to transparently manage and
/// inspect service authorization state across the cluster.
/// </summary>
public sealed class ServiceAuthzRegistration
{
    /// <summary>Service identity (e.g., "catalog.CatalogService").</summary>
    [JsonPropertyName("service_name")]
    public string ServiceName { get; set; } = "";

    /// <summary>Service version.</summary>
    [JsonPropertyName("service_version")]
    public string? ServiceVersion { get; set; }

    /// <summary>Node/instance identifier.</summary>
    [JsonPropertyName("node_id")]
    public string? NodeId { get; set; }

    // ── Network ─────────────────────────────────────────────────────────

    /// <summary>gRPC endpoint address.</summary>
    [JsonPropertyName("grpc_address")]
    public string? GrpcAddress { get; set; }

    /// <summary>HTTP endpoint address (if applicable).</summary>
    [JsonPropertyName("http_address")]
    public string? HttpAddress { get; set; }

    // ── Authorization state ─────────────────────────────────────────────

    /// <summary>Configured authorization mode (RbacStrict/Bootstrap/Development).</summary>
    [JsonPropertyName("authz_mode")]
    public string AuthzMode { get; set; } = "RbacStrict";

    /// <summary>Whether RBAC gRPC validation is currently active and reachable.</summary>
    [JsonPropertyName("rbac_active")]
    public bool RbacActive { get; set; }

    /// <summary>Whether the service is in degraded/fallback authorization mode.</summary>
    [JsonPropertyName("fallback_active")]
    public bool FallbackActive { get; set; }

    /// <summary>Whether authorization manifests are loaded.</summary>
    [JsonPropertyName("manifests_loaded")]
    public bool ManifestsLoaded { get; set; }

    // ── Manifest metadata ───────────────────────────────────────────────

    /// <summary>Source of the loaded permissions manifest (file path or "compiled").</summary>
    [JsonPropertyName("permissions_source")]
    public string? PermissionsSource { get; set; }

    /// <summary>Schema version of the loaded manifest.</summary>
    [JsonPropertyName("manifest_schema_version")]
    public string? ManifestSchemaVersion { get; set; }

    /// <summary>Number of permission entries loaded.</summary>
    [JsonPropertyName("permission_count")]
    public int PermissionCount { get; set; }

    /// <summary>Number of method→action mappings registered in the resolver.</summary>
    [JsonPropertyName("action_mapping_count")]
    public int ActionMappingCount { get; set; }

    // ── Role seeding ────────────────────────────────────────────────────

    /// <summary>Source of the roles manifest used for seeding.</summary>
    [JsonPropertyName("roles_source")]
    public string? RolesSource { get; set; }

    /// <summary>Result of the last role seeding operation.</summary>
    [JsonPropertyName("role_seed_status")]
    public string? RoleSeedStatus { get; set; }

    // ── Timestamps ──────────────────────────────────────────────────────

    /// <summary>When the service registered / last updated its state.</summary>
    [JsonPropertyName("registered_at")]
    public DateTime RegisteredAt { get; set; } = DateTime.UtcNow;
}
