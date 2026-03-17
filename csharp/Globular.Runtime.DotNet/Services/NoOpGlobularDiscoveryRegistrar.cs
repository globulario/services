using Microsoft.Extensions.Logging;

namespace Globular.Runtime;

/// <summary>
/// Default no-op discovery registrar. Replace with a real implementation to enable service discovery.
/// </summary>
public class NoOpGlobularDiscoveryRegistrar : IGlobularDiscoveryRegistrar
{
    private readonly ILogger<NoOpGlobularDiscoveryRegistrar> _logger;

    public NoOpGlobularDiscoveryRegistrar(ILogger<NoOpGlobularDiscoveryRegistrar> logger)
    {
        _logger = logger;
    }

    public Task RegisterAsync(CancellationToken cancellationToken = default)
    {
        _logger.LogDebug("Discovery registration skipped (no-op registrar)");
        return Task.CompletedTask;
    }

    public Task UnregisterAsync(CancellationToken cancellationToken = default)
    {
        _logger.LogDebug("Discovery unregistration skipped (no-op registrar)");
        return Task.CompletedTask;
    }
}
