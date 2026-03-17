namespace Globular.Runtime;

/// <summary>
/// Registers/unregisters the service with a discovery system.
/// </summary>
public interface IGlobularDiscoveryRegistrar
{
    Task RegisterAsync(CancellationToken cancellationToken = default);
    Task UnregisterAsync(CancellationToken cancellationToken = default);
}
