// PolicyLoader — mirrors Go policy.LoadPermissions() and policy.LoadServiceRoles().
// Loads authorization policy from external JSON files with precedence:
//   1. /etc/globular/policy/           (admin override)
//   2. /var/lib/globular/policy/       (package-shipped generated)
//   3. compiled fallback               (service-provided defaults)

using System.Text.Json;
using Microsoft.Extensions.Logging;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Loads permission and role manifests from the filesystem with precedence rules.
/// Thread-safe and suitable for DI registration as a singleton.
/// </summary>
public sealed class PolicyLoader
{
    private readonly ILogger<PolicyLoader> _logger;

    /// <summary>Admin override root. Overridable for testing.</summary>
    public static string AdminRoot { get; set; } = "/etc/globular/policy";

    /// <summary>Package-shipped default root. Overridable for testing.</summary>
    public static string PackageRoot { get; set; } = "/var/lib/globular/policy";

    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        PropertyNameCaseInsensitive = true,
        ReadCommentHandling = JsonCommentHandling.Skip,
    };

    public PolicyLoader(ILogger<PolicyLoader> logger)
    {
        _logger = logger;
    }

    /// <summary>
    /// Loads permissions for a service. Returns null if no file found (caller
    /// should use compiled fallback).
    /// </summary>
    public PermissionsManifest? LoadPermissions(string serviceName)
    {
        var paths = PermissionsPaths(serviceName);
        var (data, path) = ReadFirst(paths);
        if (data is null) return null;

        try
        {
            var manifest = JsonSerializer.Deserialize<PermissionsManifest>(data, JsonOpts);
            if (manifest is null || manifest.Permissions.Count == 0)
            {
                _logger.LogWarning("Policy: empty permissions manifest at {Path}", path);
                return null;
            }

            var errors = ValidatePermissions(manifest);
            if (errors.Count > 0)
            {
                foreach (var err in errors)
                    _logger.LogError("Policy: permissions validation error in {Path}: {Error}", path, err);
                return null;
            }

            _logger.LogInformation("Policy: loaded permissions from {Path} ({Count} entries)",
                path, manifest.Permissions.Count);
            return manifest;
        }
        catch (JsonException ex)
        {
            _logger.LogWarning("Policy: invalid JSON in {Path}: {Error}", path, ex.Message);
            return null;
        }
    }

    /// <summary>
    /// Loads service roles for seeding. Returns null if no file found.
    /// </summary>
    public RolesManifest? LoadServiceRoles(string serviceName)
    {
        var paths = RolesPaths(serviceName);
        var (data, path) = ReadFirst(paths);
        if (data is null) return null;

        try
        {
            var manifest = JsonSerializer.Deserialize<RolesManifest>(data, JsonOpts);
            if (manifest is null || manifest.Roles.Count == 0)
            {
                _logger.LogWarning("Policy: empty roles manifest at {Path}", path);
                return null;
            }

            _logger.LogInformation("Policy: loaded service roles from {Path} ({Count} roles)",
                path, manifest.Roles.Count);
            return manifest;
        }
        catch (JsonException ex)
        {
            _logger.LogWarning("Policy: invalid JSON in {Path}: {Error}", path, ex.Message);
            return null;
        }
    }

    // ── Path helpers ────────────────────────────────────────────────────────

    private static string[] PermissionsPaths(string serviceName) => new[]
    {
        Path.Combine(AdminRoot, "services", serviceName, "permissions.json"),
        Path.Combine(PackageRoot, "services", serviceName, "permissions.generated.json"),
        Path.Combine(PackageRoot, "services", serviceName, "permissions.json"),
    };

    private static string[] RolesPaths(string serviceName) => new[]
    {
        Path.Combine(AdminRoot, "services", serviceName, "roles.json"),
        Path.Combine(PackageRoot, "services", serviceName, "roles.generated.json"),
        Path.Combine(PackageRoot, "services", serviceName, "roles.json"),
    };

    private static (string? data, string? path) ReadFirst(string[] paths)
    {
        foreach (var p in paths)
        {
            try
            {
                if (File.Exists(p))
                    return (File.ReadAllText(p), p);
            }
            catch { /* permission denied, etc. — try next */ }
        }
        return (null, null);
    }

    // ── Validation ──────────────────────────────────────────────────────────

    private static readonly HashSet<string> ValidVerbs = new()
        { "read", "write", "delete", "admin" };

    private static List<string> ValidatePermissions(PermissionsManifest manifest)
    {
        var errors = new List<string>();
        if (string.IsNullOrEmpty(manifest.Version) && string.IsNullOrEmpty(manifest.SchemaVersion))
            errors.Add("missing version or schema_version field");
        if (string.IsNullOrEmpty(manifest.Service))
            errors.Add("missing service field");

        var seenMethods = new HashSet<string>();
        for (int i = 0; i < manifest.Permissions.Count; i++)
        {
            var p = manifest.Permissions[i];
            var prefix = $"permissions[{i}]";

            if (!string.IsNullOrEmpty(p.Method))
            {
                // v2 format
                if (!p.Method.StartsWith('/'))
                    errors.Add($"{prefix}: method must start with /");
                if (!seenMethods.Add(p.Method))
                    errors.Add($"{prefix}: duplicate method {p.Method}");
                if (string.IsNullOrEmpty(p.Action))
                    errors.Add($"{prefix}: missing action key");
                else if (!IsActionKey(p.Action))
                    errors.Add($"{prefix}: invalid action key \"{p.Action}\"");
            }
            else
            {
                // v1 format
                if (string.IsNullOrEmpty(p.Action))
                    errors.Add($"{prefix}: missing action");
                else if (!p.Action.StartsWith('/'))
                    errors.Add($"{prefix}: v1 action must start with /");
            }

            if (!string.IsNullOrEmpty(p.Permission) && !ValidVerbs.Contains(p.Permission))
                errors.Add($"{prefix}: invalid permission verb \"{p.Permission}\"");
        }

        return errors;
    }

    /// <summary>
    /// Returns true if the string is a stable action key (lowercase dotted identifier).
    /// </summary>
    public static bool IsActionKey(string s)
    {
        if (string.IsNullOrEmpty(s) || s.StartsWith('/'))
            return false;
        // Must contain at least one dot, all lowercase alphanumeric + dots
        if (!s.Contains('.'))
            return false;
        foreach (var c in s)
        {
            if (c != '.' && !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9'))
                return false;
        }
        return s[0] >= 'a' && s[0] <= 'z'; // must start with letter
    }
}
