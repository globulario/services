using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Logging;

namespace Globular.Runtime;

public static class ServiceCollectionExtensions
{
    /// <summary>
    /// Adds Globular runtime services: options binding, service context, manifest provider,
    /// health, startup pipeline, and the hosted startup service.
    /// Startup runs exactly once via GlobularHostedStartupService.
    /// </summary>
    public static IServiceCollection AddGlobularRuntime(
        this IServiceCollection services,
        IConfiguration configuration)
    {
        // Bind options from config section "Globular"
        services.Configure<GlobularHostOptions>(configuration.GetSection("Globular"));

        // Core runtime services
        services.TryAddSingleton<Authorization.EffectiveRuntimeState>();
        services.TryAddSingleton<IGlobularServiceContext, GlobularServiceContext>();
        services.TryAddSingleton<IGlobularHealthReporter, GlobularHealthReporter>();
        services.TryAddSingleton<IGlobularManifestProvider, GlobularManifestProvider>();
        services.TryAddSingleton<GlobularRuntimeBootstrapper>();

        // Built-in startup task: manifest validation
        services.AddSingleton<IGlobularStartupTask, ManifestValidationStartupTask>();

        // Startup hosted service — single startup orchestration point
        services.AddHostedService<GlobularHostedStartupService>();

        return services;
    }

    /// <summary>
    /// Adds Globular health reporting support.
    /// </summary>
    public static IServiceCollection AddGlobularHealth(
        this IServiceCollection services,
        IConfiguration configuration)
    {
        services.Configure<GlobularHealthOptions>(configuration.GetSection("Globular:Health"));
        services.TryAddSingleton<IGlobularHealthReporter, GlobularHealthReporter>();
        return services;
    }

    /// <summary>
    /// Adds Globular discovery registration support.
    ///
    /// Managed mode (Discovery:Enabled=true, default):
    ///   Real GlobularServiceRegistrar backed by EtcdServiceStatePublisher.
    ///   Service state published to /globular/services/{id}/runtime-state.
    ///
    /// Local/test mode (Discovery:Enabled=false):
    ///   NoOp registrar — no cluster registration.
    /// </summary>
    public static IServiceCollection AddGlobularDiscovery(
        this IServiceCollection services,
        IConfiguration configuration)
    {
        services.Configure<GlobularDiscoveryOptions>(configuration.GetSection("Globular:Discovery"));

        var discoveryEnabled = configuration.GetValue<bool>("Globular:Discovery:Enabled");
        if (discoveryEnabled)
        {
            // Managed mode: real registrar backed by etcd.
            // If AddGlobularAuthorization was called first, IServiceStatePublisher
            // is already registered (EtcdServiceStatePublisher). Otherwise, fall
            // back to LoggingServiceStatePublisher as a visible indicator.
            services.TryAddSingleton<Authorization.IServiceStatePublisher,
                Authorization.LoggingServiceStatePublisher>();
            services.TryAddSingleton<IGlobularDiscoveryRegistrar>(sp =>
                new Authorization.GlobularServiceRegistrar(
                    sp.GetRequiredService<Authorization.IServiceStatePublisher>(),
                    sp.GetRequiredService<Authorization.EffectiveRuntimeState>(),
                    sp.GetRequiredService<ILogger<Authorization.GlobularServiceRegistrar>>()));
        }
        else
        {
            // Local/test mode: no-op registrar.
            services.TryAddSingleton<IGlobularDiscoveryRegistrar, NoOpGlobularDiscoveryRegistrar>();
        }

        return services;
    }

    /// <summary>
    /// Registers a startup task to be executed during application boot.
    /// </summary>
    public static IServiceCollection AddGlobularStartupTask<T>(this IServiceCollection services)
        where T : class, IGlobularStartupTask
    {
        services.AddSingleton<IGlobularStartupTask, T>();
        return services;
    }
}
