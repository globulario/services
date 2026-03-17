// Policy data models — mirrors Go policy package types.
// These are the JSON-deserializable types for permissions.json,
// permissions.generated.json, roles.json, and roles.generated.json.

using System.Text.Json.Serialization;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Top-level permissions manifest (permissions.json / permissions.generated.json).
/// </summary>
public sealed class PermissionsManifest
{
    [JsonPropertyName("version")]
    public string? Version { get; set; }

    [JsonPropertyName("schema_version")]
    public string? SchemaVersion { get; set; }

    [JsonPropertyName("generator_version")]
    public string? GeneratorVersion { get; set; }

    [JsonPropertyName("service")]
    public string Service { get; set; } = "";

    [JsonPropertyName("permissions")]
    public List<PermissionEntry> Permissions { get; set; } = new();
}

/// <summary>
/// A single permission entry mapping a gRPC method to a stable action key.
/// </summary>
public sealed class PermissionEntry
{
    /// <summary>gRPC full method path (e.g., "/catalog.CatalogService/GetItemDefinition").</summary>
    [JsonPropertyName("method")]
    public string? Method { get; set; }

    /// <summary>Stable RBAC action key (e.g., "catalog.item_definition.read").</summary>
    [JsonPropertyName("action")]
    public string Action { get; set; } = "";

    /// <summary>Permission kind: "read", "write", "delete", "admin".</summary>
    [JsonPropertyName("permission")]
    public string? Permission { get; set; }

    /// <summary>Resource template with {field} placeholders for individual resources.</summary>
    [JsonPropertyName("resource_template")]
    public string? ResourceTemplate { get; set; }

    /// <summary>Collection template for list/create operations.</summary>
    [JsonPropertyName("collection_template")]
    public string? CollectionTemplate { get; set; }

    /// <summary>Resource extraction rules (legacy field-index-based).</summary>
    [JsonPropertyName("resources")]
    public List<ResourceEntry> Resources { get; set; } = new();
}

/// <summary>
/// Resource field extraction metadata.
/// </summary>
public sealed class ResourceEntry
{
    [JsonPropertyName("field")]
    public string Field { get; set; } = "";

    [JsonPropertyName("kind")]
    public string? Kind { get; set; }

    [JsonPropertyName("scope_anchor")]
    public bool ScopeAnchor { get; set; }

    [JsonPropertyName("index")]
    public int Index { get; set; }

    [JsonPropertyName("permission")]
    public string? Permission { get; set; }
}

/// <summary>
/// Top-level roles manifest (roles.json / roles.generated.json).
/// </summary>
public sealed class RolesManifest
{
    [JsonPropertyName("version")]
    public string? Version { get; set; }

    [JsonPropertyName("schema_version")]
    public string? SchemaVersion { get; set; }

    [JsonPropertyName("generator_version")]
    public string? GeneratorVersion { get; set; }

    [JsonPropertyName("service")]
    public string Service { get; set; } = "";

    [JsonPropertyName("roles")]
    public List<RoleEntry> Roles { get; set; } = new();
}

/// <summary>
/// A default role definition for seeding into RBAC.
/// </summary>
public sealed class RoleEntry
{
    [JsonPropertyName("name")]
    public string Name { get; set; } = "";

    [JsonPropertyName("inherits")]
    public List<string>? Inherits { get; set; }

    [JsonPropertyName("actions")]
    public List<string> Actions { get; set; } = new();
}
