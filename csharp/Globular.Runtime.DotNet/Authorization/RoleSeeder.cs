// RoleSeeder — mirrors Go policy.SeedServiceRoles().
// Seeds missing roles into RBAC from generated manifests.
// Never overwrites existing roles (admin-edited roles are preserved).

using Microsoft.Extensions.Logging;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Interface for RBAC role persistence. Implement this to connect to the
/// Globular RBAC service via gRPC.
/// </summary>
public interface IRoleStore
{
    /// <summary>Returns true if a role with the given name exists in RBAC.</summary>
    Task<bool> RoleExistsAsync(string roleName, CancellationToken ct = default);

    /// <summary>
    /// Creates a new role. Must not overwrite existing roles.
    /// Metadata contains provenance (source, service, seeded_at).
    /// </summary>
    Task CreateRoleAsync(string roleName, IReadOnlyList<string> actions,
        IReadOnlyDictionary<string, string> metadata, CancellationToken ct = default);
}

/// <summary>
/// Result of a role seeding operation.
/// </summary>
public sealed record SeedResult(int Seeded, int Skipped, int Failed);

/// <summary>
/// Seeds missing roles into RBAC from generated/override manifest files.
/// Existing roles are never overwritten — admin edits survive service
/// reload, package regeneration, node restart, and upgrade.
/// </summary>
public sealed class RoleSeeder
{
    private readonly PolicyLoader _loader;
    private readonly IRoleStore _store;
    private readonly ILogger<RoleSeeder> _logger;

    public RoleSeeder(PolicyLoader loader, IRoleStore store, ILogger<RoleSeeder> logger)
    {
        _loader = loader;
        _store = store;
        _logger = logger;
    }

    /// <summary>
    /// Seeds missing roles for a service. Returns counts of seeded/skipped/failed.
    /// </summary>
    public async Task<SeedResult> SeedAsync(string serviceName, CancellationToken ct = default)
    {
        var manifest = _loader.LoadServiceRoles(serviceName);
        if (manifest is null || manifest.Roles.Count == 0)
            return new SeedResult(0, 0, 0);

        int seeded = 0, skipped = 0, failed = 0;

        foreach (var role in manifest.Roles)
        {
            try
            {
                if (await _store.RoleExistsAsync(role.Name, ct))
                {
                    // Role exists in RBAC (possibly admin-edited) — preserve it.
                    skipped++;
                    continue;
                }

                var metadata = new Dictionary<string, string>
                {
                    ["source"] = "generated",
                    ["service"] = serviceName,
                    ["managed"] = "seed",
                    ["seeded_at"] = DateTime.UtcNow.ToString("o"),
                };

                await _store.CreateRoleAsync(role.Name, role.Actions, metadata, ct);
                _logger.LogInformation("Policy: seeded missing role {Role} ({ActionCount} actions)",
                    role.Name, role.Actions.Count);
                seeded++;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Policy: failed to seed role {Role}", role.Name);
                failed++;
            }
        }

        return new SeedResult(seeded, skipped, failed);
    }
}
