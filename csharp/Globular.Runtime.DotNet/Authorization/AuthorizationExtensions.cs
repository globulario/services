// DI registration extensions for Globular authorization components.
// Usage:
//   builder.Services.AddGlobularAuthorization("catalog");
//   builder.Services.AddGrpc(o => o.Interceptors.Add<GlobularAuthorizationInterceptor>());

using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace Globular.Runtime.Authorization;

public static class AuthorizationExtensions
{
    /// <summary>
    /// Registers Globular authorization services: PolicyLoader, ActionResolver,
    /// and GlobularAuthorizationInterceptor. Call before AddGrpc().
    /// </summary>
    /// <param name="services">The service collection.</param>
    /// <param name="serviceName">The service name for loading policy files (e.g., "catalog").</param>
    /// <returns>The service collection for chaining.</returns>
    public static IServiceCollection AddGlobularAuthorization(
        this IServiceCollection services, string serviceName)
    {
        services.AddSingleton<PolicyLoader>();
        services.AddSingleton<ActionResolver>(sp =>
        {
            var loader = sp.GetRequiredService<PolicyLoader>();
            var resolver = new ActionResolver();

            // Load permissions and register method→action mappings.
            var manifest = loader.LoadPermissions(serviceName);
            if (manifest is not null)
                resolver.Register(manifest);

            return resolver;
        });
        services.AddSingleton<GlobularAuthorizationInterceptor>();

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
        if (store is null) return; // no RBAC store registered

        var logger = services.GetRequiredService<ILoggerFactory>()
            .CreateLogger<RoleSeeder>();
        var seeder = new RoleSeeder(loader, store, logger);
        var result = await seeder.SeedAsync(serviceName, ct);

        logger.LogInformation("Role seeding complete: {Seeded} seeded, {Skipped} preserved, {Failed} failed",
            result.Seeded, result.Skipped, result.Failed);
    }
}
