// AuthorizationStartupTask — integrates authorization initialization into
// the Globular managed-service startup pipeline.
//
// This task runs during service startup (via IGlobularStartupTask) and:
//   1. Loads permission manifests from generated/override files
//   2. Populates the ActionResolver with method→action mappings
//   3. Seeds missing roles into RBAC (non-destructive)
//   4. Publishes effective authz state for service registration

using Globular.Runtime.Abstractions;
using Microsoft.Extensions.Logging;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Globular startup task that initializes authorization from manifests
/// and seeds missing roles into RBAC. Fits the managed-service startup
/// pipeline (IGlobularStartupTask).
/// </summary>
public sealed class AuthorizationStartupTask : IGlobularStartupTask
{
    private readonly string _serviceName;
    private readonly PolicyLoader _loader;
    private readonly ActionResolver _resolver;
    private readonly IRoleStore? _roleStore;
    private readonly ILogger<AuthorizationStartupTask> _logger;

    /// <summary>Effective authorization state, available for service registration.</summary>
    public ServiceAuthzRegistration Registration { get; } = new();

    /// <summary>Run early in startup, after config but before gRPC handlers.</summary>
    public int Order => 10;

    public AuthorizationStartupTask(
        string serviceName,
        PolicyLoader loader,
        ActionResolver resolver,
        ILogger<AuthorizationStartupTask> logger,
        IRoleStore? roleStore = null)
    {
        _serviceName = serviceName;
        _loader = loader;
        _resolver = resolver;
        _roleStore = roleStore;
        _logger = logger;
        Registration.ServiceName = serviceName;
    }

    public async Task ExecuteAsync(CancellationToken ct)
    {
        _logger.LogInformation("Authz startup: initializing authorization for {Service}", _serviceName);

        // 1. Load permission manifest.
        var manifest = _loader.LoadPermissions(_serviceName);
        if (manifest is not null)
        {
            _resolver.Register(manifest);
            Registration.ManifestsLoaded = true;
            Registration.PermissionCount = manifest.Permissions.Count;
            Registration.ManifestSchemaVersion = manifest.SchemaVersion ?? manifest.Version;
            _logger.LogInformation("Authz startup: loaded {Count} permission entries from manifest",
                manifest.Permissions.Count);
        }
        else
        {
            _logger.LogInformation("Authz startup: no permission manifest found, using compiled fallback");
            Registration.PermissionsSource = "compiled";
        }

        // 2. Seed missing roles (non-destructive).
        if (_roleStore is not null)
        {
            var seeder = new RoleSeeder(_loader, _roleStore,
                new Logger<RoleSeeder>(new LoggerFactory()));
            var result = await seeder.SeedAsync(_serviceName, ct);
            Registration.RoleSeedStatus = $"seeded={result.Seeded},skipped={result.Skipped},failed={result.Failed}";
            _logger.LogInformation("Authz startup: role seeding complete ({Status})",
                Registration.RoleSeedStatus);
        }

        Registration.RegisteredAt = DateTime.UtcNow;
        _logger.LogInformation("Authz startup: authorization initialized for {Service}", _serviceName);
    }
}
