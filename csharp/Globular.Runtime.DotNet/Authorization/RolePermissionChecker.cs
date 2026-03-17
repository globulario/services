// RolePermissionChecker — mirrors Go security/roles.go HasRolePermission().
// Checks whether a subject's role binding grants access to a gRPC method
// or stable action key.
//
// Go reference: golang/security/roles.go
//
// Role permission data is loaded from policy files (roles.json) at startup.
// Compiled defaults (globular-admin = "/*") are always present.

namespace Globular.Runtime.Authorization;

/// <summary>
/// Checks whether a set of roles grants permission to perform an action.
/// Mirrors Go security.HasRolePermission() with wildcard and prefix support.
///
/// Supports:
///   - Exact match: "file.read" == "file.read"
///   - Global wildcard: "*" or "/*" grants all
///   - Action-key wildcard: "file.*" matches "file.read"
///   - Method-path wildcard: "/pkg.Service/*" matches "/pkg.Service/Method"
/// </summary>
public sealed class RolePermissionChecker
{
    /// <summary>
    /// Maps role names to the set of actions/methods each role is allowed to call.
    /// Mirrors Go security.RolePermissions.
    /// </summary>
    private readonly Dictionary<string, List<string>> _rolePermissions = new();

    public RolePermissionChecker()
    {
        // Compiled default: globular-admin has unrestricted access.
        // Mirrors Go: RoleAdmin: {"/*"}
        _rolePermissions["globular-admin"] = new List<string> { "/*" };
    }

    /// <summary>
    /// Registers role permissions from a loaded roles manifest.
    /// Called during startup after loading roles.json / roles.generated.json.
    /// </summary>
    public void RegisterFromManifest(RolesManifest manifest)
    {
        foreach (var role in manifest.Roles)
        {
            if (string.IsNullOrEmpty(role.Name) || role.Actions.Count == 0)
                continue;
            _rolePermissions[role.Name] = new List<string>(role.Actions);
        }
    }

    /// <summary>
    /// Checks if any of the given roles grants access to the specified action.
    /// Mirrors Go security.HasRolePermission(roles, action).
    /// </summary>
    public bool HasPermission(IReadOnlyList<string> roles, string action)
    {
        foreach (var role in roles)
        {
            if (!_rolePermissions.TryGetValue(role, out var permissions))
                continue;

            foreach (var perm in permissions)
            {
                if (MatchesPermission(perm, action))
                    return true;
            }
        }
        return false;
    }

    /// <summary>
    /// Checks if a single permission grant matches an action.
    /// Mirrors Go security.matchesPermission().
    /// </summary>
    private static bool MatchesPermission(string perm, string action)
    {
        // Global wildcards
        if (perm == "*" || perm == "/*")
            return true;

        // Exact match
        if (perm == action)
            return true;

        // Action-key wildcard: "file.*" matches "file.read"
        if (perm.EndsWith(".*"))
        {
            var prefix = perm[..^1]; // strip trailing *
            if (action.StartsWith(prefix))
                return true;
        }

        // Method-path wildcard: "/pkg.Service/*" matches "/pkg.Service/Method"
        if (perm.EndsWith("/*"))
        {
            var prefix = perm[..^1]; // strip trailing *
            if (action.StartsWith(prefix))
                return true;
        }

        return false;
    }
}
