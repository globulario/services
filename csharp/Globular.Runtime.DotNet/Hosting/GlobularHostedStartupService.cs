using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime;

/// <summary>
/// Hosted service that runs the Globular startup pipeline when the application starts,
/// performs discovery registration if enabled, and unregisters on shutdown.
/// This is the single startup orchestration point — do not also call UseGlobularRuntimeAsync().
/// </summary>
public class GlobularHostedStartupService : IHostedService
{
    private readonly GlobularRuntimeBootstrapper _bootstrapper;
    private readonly IGlobularDiscoveryRegistrar _discovery;
    private readonly GlobularHostOptions _options;
    private readonly ILogger<GlobularHostedStartupService> _logger;

    public GlobularHostedStartupService(
        GlobularRuntimeBootstrapper bootstrapper,
        IGlobularDiscoveryRegistrar discovery,
        IOptions<GlobularHostOptions> options,
        ILogger<GlobularHostedStartupService> logger)
    {
        _bootstrapper = bootstrapper;
        _discovery = discovery;
        _options = options.Value;
        _logger = logger;
    }

    public async Task StartAsync(CancellationToken cancellationToken)
    {
        // Run validation, startup tasks, set health
        await _bootstrapper.RunAsync(cancellationToken);

        // Discovery registration
        if (_options.Discovery.Enabled && _options.Discovery.RegisterOnStartup)
        {
            try
            {
                _logger.LogInformation("Registering with discovery service...");
                await _discovery.RegisterAsync(cancellationToken);
                _logger.LogInformation("Discovery registration complete");
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Discovery registration failed");
            }
        }
    }

    public async Task StopAsync(CancellationToken cancellationToken)
    {
        if (_options.Discovery.Enabled)
        {
            try
            {
                await _discovery.UnregisterAsync(cancellationToken);
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Discovery unregistration failed during shutdown");
            }
        }
    }
}
