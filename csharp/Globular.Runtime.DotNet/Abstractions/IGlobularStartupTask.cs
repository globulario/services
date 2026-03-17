namespace Globular.Runtime;

/// <summary>
/// A task executed during application startup (e.g., validation, registration, warmup).
/// </summary>
public interface IGlobularStartupTask
{
    string Name { get; }
    int Order { get; }
    Task ExecuteAsync(CancellationToken cancellationToken = default);
}
