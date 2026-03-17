// AuthorizationMode — controls runtime authorization behavior.
// Default is RbacStrict (fail closed if RBAC unavailable).

namespace Globular.Runtime.Authorization;

/// <summary>
/// Controls how the service handles authorization at runtime.
/// </summary>
public enum AuthorizationMode
{
    /// <summary>
    /// Production mode: RBAC gRPC is the authority.
    /// If RBAC is unavailable, protected operations fail closed.
    /// This is the default and recommended mode.
    /// </summary>
    RbacStrict,

    /// <summary>
    /// Bootstrap mode: used during Day-0 cluster initialization.
    /// Local manifest-based authorization is used as fallback when RBAC
    /// is not yet available. Logs loudly that the service is in degraded
    /// authz mode.
    /// </summary>
    Bootstrap,

    /// <summary>
    /// Development mode: local manifest-based authorization only.
    /// RBAC gRPC is not required. For local development only.
    /// Logs warning on every request.
    /// </summary>
    Development,
}

/// <summary>
/// Runtime authorization state for a service instance.
/// Published to etcd as part of service registration so Globular
/// can inspect effective authz configuration cluster-wide.
/// </summary>
public sealed class AuthorizationState
{
    /// <summary>Configured authorization mode.</summary>
    public AuthorizationMode Mode { get; set; } = AuthorizationMode.RbacStrict;

    /// <summary>Whether RBAC gRPC validation is currently active.</summary>
    public bool RbacActive { get; set; }

    /// <summary>Whether the service is in degraded/fallback authz mode.</summary>
    public bool FallbackActive { get; set; }

    /// <summary>Source of loaded permissions manifest (file path or "compiled").</summary>
    public string? PermissionsSource { get; set; }

    /// <summary>Schema version of loaded manifest.</summary>
    public string? ManifestSchemaVersion { get; set; }

    /// <summary>Number of permission entries loaded.</summary>
    public int PermissionCount { get; set; }

    /// <summary>Number of method→action mappings registered.</summary>
    public int ActionMappingCount { get; set; }

    /// <summary>Role seed result from last startup.</summary>
    public string? RoleSeedStatus { get; set; }

    /// <summary>Timestamp of last authorization state update.</summary>
    public DateTime LastUpdated { get; set; } = DateTime.UtcNow;
}
