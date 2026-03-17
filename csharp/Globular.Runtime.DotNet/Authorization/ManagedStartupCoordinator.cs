// ManagedStartupCoordinator — orchestrates the Globular managed-service
// startup sequence mirroring the Go service startup pattern.
//
// Go reference: globular_service/services.go InitService() + lifecycle.go Start()
//
// Startup order (mirrors Go):
//   1. Initialize service defaults
//   2. Generate service ID if missing
//   3. Resolve/select ports
//   4. Resolve address/domain
//   5. Load runtime config
//   6. Load permission manifests + populate action resolver
//   7. Initialize RBAC connection
//   8. Seed missing roles (non-destructive)
//   9. Publish effective runtime state
//  10. Transition to ready

using Globular.Runtime.Abstractions;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Orchestrates the full managed-service startup sequence.
/// Implements IGlobularStartupTask so it integrates with the existing
/// startup pipeline. Runs at Order=5 (before handler registration).
/// </summary>
public sealed class ManagedStartupCoordinator : IGlobularStartupTask
{
    private readonly string _serviceName;
    private readonly PolicyLoader _loader;
    private readonly ActionResolver _resolver;
    private readonly RuntimeResolver _runtimeResolver;
    private readonly EffectiveRuntimeState _state;
    private readonly IGlobularDiscoveryRegistrar _registrar;
    private readonly IRoleStore? _roleStore;
    private readonly AuthorizationMode _mode;
    private readonly IOptions<GlobularHostOptions> _hostOptions;
    private readonly ILogger<ManagedStartupCoordinator> _logger;
    private readonly ILoggerFactory _loggerFactory;

    public int Order => 5; // Early in startup, after config validation

    public ManagedStartupCoordinator(
        string serviceName,
        PolicyLoader loader,
        ActionResolver resolver,
        RuntimeResolver runtimeResolver,
        EffectiveRuntimeState state,
        IGlobularDiscoveryRegistrar registrar,
        IOptions<GlobularHostOptions> hostOptions,
        ILogger<ManagedStartupCoordinator> logger,
        AuthorizationMode mode = AuthorizationMode.RbacStrict,
        IRoleStore? roleStore = null,
        ILoggerFactory? loggerFactory = null)
    {
        _serviceName = serviceName;
        _loader = loader;
        _resolver = resolver;
        _runtimeResolver = runtimeResolver;
        _state = state;
        _registrar = registrar;
        _roleStore = roleStore;
        _mode = mode;
        _hostOptions = hostOptions;
        _logger = logger;
        _loggerFactory = loggerFactory ?? new LoggerFactory();
    }

    public async Task ExecuteAsync(CancellationToken ct)
    {
        _logger.LogInformation("Managed startup: beginning for {Service}", _serviceName);

        // 1-4. Resolve runtime identity, ports, address, domain.
        // (Already done by RuntimeResolver when EffectiveRuntimeState was created
        // via DI — just log the resolved values.)
        _logger.LogInformation(
            "Managed startup: identity={Id} address={Address}:{Port} domain={Domain}",
            _state.ServiceId, _state.GrpcAddress, _state.GrpcPort, _state.Domain);

        // 5. Load runtime config (already bound via IOptions<GlobularHostOptions>).

        // 6. Load permission manifests and populate action resolver.
        var manifest = _loader.LoadPermissions(_serviceName);
        if (manifest is not null)
        {
            _resolver.Register(manifest);
            _state.ManifestsLoaded = true;
            _state.PermissionCount = manifest.Permissions.Count;
            _state.ManifestSchemaVersion = manifest.SchemaVersion ?? manifest.Version;
            _state.PermissionsSource = "manifest";
            _logger.LogInformation("Managed startup: loaded {Count} permission entries",
                manifest.Permissions.Count);
        }
        else
        {
            _state.PermissionsSource = "compiled";
            _logger.LogInformation("Managed startup: no manifest found, using compiled fallback");
        }

        // 7. RBAC mode.
        _state.AuthzMode = _mode.ToString();

        // 8. Seed missing roles (non-destructive).
        if (_roleStore is not null)
        {
            var seeder = new RoleSeeder(_loader, _roleStore,
                _loggerFactory.CreateLogger<RoleSeeder>());
            var result = await seeder.SeedAsync(_serviceName, ct);
            _state.RoleSeedStatus =
                $"seeded={result.Seeded},skipped={result.Skipped},failed={result.Failed}";
            _logger.LogInformation("Managed startup: role seeding: {Status}", _state.RoleSeedStatus);
        }

        // 9. Publish effective runtime state.
        _state.Readiness = "ready";
        _state.LastHeartbeat = DateTime.UtcNow;
        await _registrar.RegisterAsync(ct);

        // 10. Done.
        _logger.LogInformation("Managed startup: {Service} is ready", _serviceName);
    }
}
