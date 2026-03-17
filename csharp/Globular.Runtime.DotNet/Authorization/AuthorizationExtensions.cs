// DI registration extensions for Globular authorization components.
// Integrates with the managed-service startup pipeline.
//
// Usage:
//   builder.Services.AddGlobularAuthorization("catalog", builder.Configuration);
//   builder.Services.AddGrpc(o => o.Interceptors.Add<GlobularAuthorizationInterceptor>());
//   // At startup:
//   await app.Services.SeedRolesAsync("catalog");
//   // Management endpoint:
//   app.MapGlobularManagement();

using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;
using System.Text.Json;

namespace Globular.Runtime.Authorization;

public static class AuthorizationExtensions
{
    /// <summary>
    /// Registers all Globular authorization services: PolicyLoader, ActionResolver,
    /// RuntimeResolver, EffectiveRuntimeState, interceptor, and registrar.
    /// </summary>
    public static IServiceCollection AddGlobularAuthorization(
        this IServiceCollection services,
        string serviceName,
        IConfiguration? configuration = null,
        AuthorizationMode mode = AuthorizationMode.RbacStrict)
    {
        services.AddSingleton<PolicyLoader>();
        services.AddSingleton<IPortAllocator, DefaultPortAllocator>();
        services.AddSingleton<RuntimeResolver>();

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

        // Interceptor with configured mode.
        services.AddSingleton(sp => new GlobularAuthorizationInterceptor(
            sp.GetRequiredService<ActionResolver>(),
            sp.GetRequiredService<IRbacClient>(),
            sp.GetRequiredService<ILogger<GlobularAuthorizationInterceptor>>(),
            mode));

        // Service registrar: real if publisher available, logging fallback otherwise.
        if (!services.Any(s => s.ServiceType == typeof(IServiceStatePublisher)))
            services.AddSingleton<IServiceStatePublisher, LoggingServiceStatePublisher>();

        services.AddSingleton<IGlobularDiscoveryRegistrar>(sp =>
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
