// DI registration extensions for Globular authorization components.
// Integrates with the managed-service startup pipeline.
//
// Managed mode (default):
//   Real RBAC client, real etcd publisher, real startup coordinator.
//   Fail-closed if RBAC is unavailable.
//
// Development mode:
//   Logging publisher, no-op registrar, RBAC optional.
//   Explicit opt-in only.
//
// Usage:
//   // Managed mode (production):
//   builder.Services.AddGlobularAuthorization("catalog", builder.Configuration);
//
//   // Development mode (local only):
//   builder.Services.AddGlobularAuthorization("catalog", builder.Configuration,
//       AuthorizationMode.Development);
//
//   builder.Services.AddGrpc(o => o.Interceptors.Add<GlobularAuthorizationInterceptor>());
//   app.MapGlobularManagement();

using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;
using System.Text.Json;

namespace Globular.Runtime.Authorization;

public static class AuthorizationExtensions
{
    /// <summary>
    /// Registers all Globular authorization services for managed mode.
    ///
    /// Managed mode (RbacStrict, Bootstrap) registers:
    /// - Real GlobularRbacClient (IRbacClient + IRoleStore)
    /// - Real GlobularEtcdClient + EtcdServiceStatePublisher
    /// - ManagedStartupCoordinator for orchestrated startup
    /// - GlobularServiceRegistrar for cluster registration
    ///
    /// Development mode registers:
    /// - Real RBAC client (best-effort, non-fatal if unavailable)
    /// - LoggingServiceStatePublisher (no etcd dependency)
    /// - No ManagedStartupCoordinator
    /// </summary>
    public static IServiceCollection AddGlobularAuthorization(
        this IServiceCollection services,
        string serviceName,
        IConfiguration? configuration = null,
        AuthorizationMode mode = AuthorizationMode.RbacStrict)
    {
        // ── Core infrastructure (all modes) ──────────────────────────

        services.AddSingleton<PolicyLoader>();
        services.AddSingleton<IPortAllocator, DefaultPortAllocator>();
        services.AddSingleton<RuntimeResolver>();

        // Bind RBAC client options from config.
        if (configuration is not null)
        {
            services.Configure<RbacClientOptions>(configuration.GetSection("Globular:Rbac"));
            services.Configure<GlobularEtcdOptions>(configuration.GetSection("Globular:Etcd"));
        }

        // Resolve effective runtime state at registration time.
        services.AddSingleton<EffectiveRuntimeState>(sp =>
        {
            var resolver = sp.GetRequiredService<RuntimeResolver>();
            var options = sp.GetService<IOptions<GlobularHostOptions>>()?.Value
                ?? new GlobularHostOptions();
            return resolver.Resolve(options, mode);
        });

        // Policy loader + action resolver.
        services.AddSingleton<ActionResolver>(sp =>
        {
            var loader = sp.GetRequiredService<PolicyLoader>();
            var state = sp.GetRequiredService<EffectiveRuntimeState>();
            var actionResolver = new ActionResolver();

            var manifest = loader.LoadPermissions(serviceName);
            if (manifest is not null)
            {
                actionResolver.Register(manifest);
                state.ManifestsLoaded = true;
                state.PermissionCount = manifest.Permissions.Count;
                state.ManifestSchemaVersion = manifest.SchemaVersion ?? manifest.Version;
                state.PermissionsSource = "manifest";
            }
            else
            {
                state.PermissionsSource = "compiled";
            }

            return actionResolver;
        });

        // ── RBAC client (all modes — real gRPC client) ───────────────

        services.TryAddSingleton<GlobularRbacClient>();
        services.TryAddSingleton<IRbacClient>(sp => sp.GetRequiredService<GlobularRbacClient>());
        services.TryAddSingleton<IRoleStore>(sp => sp.GetRequiredService<GlobularRbacClient>());

        // Interceptor with configured mode.
        services.AddSingleton(sp => new GlobularAuthorizationInterceptor(
            sp.GetRequiredService<ActionResolver>(),
            sp.GetRequiredService<IRbacClient>(),
            sp.GetRequiredService<ILogger<GlobularAuthorizationInterceptor>>(),
            mode));

        // ── Mode-specific wiring ─────────────────────────────────────

        if (mode == AuthorizationMode.Development)
        {
            // Development: logging publisher, no etcd required.
            services.TryAddSingleton<IServiceStatePublisher, LoggingServiceStatePublisher>();
        }
        else
        {
            // Managed (RbacStrict, Bootstrap): real etcd publisher.
            services.TryAddSingleton<GlobularEtcdClient>();
            services.TryAddSingleton<IEtcdClient>(sp => sp.GetRequiredService<GlobularEtcdClient>());
            services.TryAddSingleton<IServiceStatePublisher, EtcdServiceStatePublisher>();

            // Register ManagedStartupCoordinator for orchestrated startup.
            services.AddSingleton<IGlobularStartupTask>(sp =>
                new ManagedStartupCoordinator(
                    serviceName,
                    sp.GetRequiredService<PolicyLoader>(),
                    sp.GetRequiredService<ActionResolver>(),
                    sp.GetRequiredService<RuntimeResolver>(),
                    sp.GetRequiredService<EffectiveRuntimeState>(),
                    sp.GetRequiredService<IGlobularDiscoveryRegistrar>(),
                    sp.GetRequiredService<IOptions<GlobularHostOptions>>(),
                    sp.GetRequiredService<ILogger<ManagedStartupCoordinator>>(),
                    mode,
                    sp.GetService<IRoleStore>(),
                    sp.GetRequiredService<ILoggerFactory>()));
        }

        // Service registrar: publishes state to management plane.
        services.TryAddSingleton<IGlobularDiscoveryRegistrar>(sp =>
            new GlobularServiceRegistrar(
                sp.GetRequiredService<IServiceStatePublisher>(),
                sp.GetRequiredService<EffectiveRuntimeState>(),
                sp.GetRequiredService<ILogger<GlobularServiceRegistrar>>()));

        return services;
    }

    /// <summary>
    /// Seeds missing roles into RBAC from generated manifest files.
    /// Call during application startup after RBAC client is available.
    /// </summary>
    public static async Task SeedRolesAsync(
        this IServiceProvider services,
        string serviceName,
        CancellationToken ct = default)
    {
        var loader = services.GetRequiredService<PolicyLoader>();
        var store = services.GetService<IRoleStore>();
        if (store is null) return;

        var logger = services.GetRequiredService<ILoggerFactory>()
            .CreateLogger<RoleSeeder>();
        var seeder = new RoleSeeder(loader, store, logger);
        var result = await seeder.SeedAsync(serviceName, ct);

        // Update effective state.
        var state = services.GetRequiredService<EffectiveRuntimeState>();
        state.RoleSeedStatus = $"seeded={result.Seeded},skipped={result.Skipped},failed={result.Failed}";
    }

    /// <summary>
    /// Maps a management endpoint that exposes the effective runtime state.
    /// GET /_globular/effective-state returns the resolved configuration as JSON.
    /// </summary>
    public static IEndpointRouteBuilder MapGlobularManagement(this IEndpointRouteBuilder endpoints)
    {
        endpoints.MapGet("/_globular/effective-state", (EffectiveRuntimeState state) =>
        {
            state.LastHeartbeat = DateTime.UtcNow;
            return Results.Json(state, new JsonSerializerOptions { WriteIndented = true });
        }).AllowAnonymous();

        return endpoints;
    }
}
