using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime;

/// <summary>
/// Orchestrates the Globular runtime startup sequence:
/// logs identity, runs startup tasks, sets health state.
/// </summary>
public class GlobularRuntimeBootstrapper
{
    private readonly IEnumerable<IGlobularStartupTask> _tasks;
    private readonly IGlobularHealthReporter _health;
    private readonly GlobularHostOptions _options;
    private readonly ILogger<GlobularRuntimeBootstrapper> _logger;

    public GlobularRuntimeBootstrapper(
        IEnumerable<IGlobularStartupTask> tasks,
        IGlobularHealthReporter health,
        IOptions<GlobularHostOptions> options,
        ILogger<GlobularRuntimeBootstrapper> logger)
    {
        _tasks = tasks;
        _health = health;
        _options = options.Value;
        _logger = logger;
    }

    public async Task RunAsync(CancellationToken cancellationToken)
    {
        var identity = _options.Service;
        var network = _options.Network;

        _logger.LogInformation("Starting Globular service: {Name} v{Version}",
            identity.Name, identity.Version);
        _logger.LogInformation("  Publisher: {Publisher}, Domain: {Domain}",
            identity.PublisherId, identity.Domain);
        _logger.LogInformation("  gRPC bind: {Address}:{Port}, TLS: {Tls}",
            network.GrpcBindAddress, network.GrpcPort, network.TlsEnabled);

        // Validate options
        var errors = GlobularOptionsValidator.Validate(_options);
        if (errors.Count > 0)
        {
            foreach (var error in errors)
            {
                _logger.LogError("Configuration error: {Error}", error);
            }
            _health.SetUnhealthy("Configuration validation failed");
            throw new InvalidOperationException(
                $"Globular configuration invalid: {string.Join("; ", errors)}");
        }

        // Execute startup tasks in order
        var orderedTasks = _tasks.OrderBy(t => t.Order).ToList();
        if (orderedTasks.Count > 0)
        {
            _logger.LogInformation("Running {Count} startup task(s)...", orderedTasks.Count);
        }

        foreach (var task in orderedTasks)
        {
            try
            {
                _logger.LogInformation("  [{Order}] {Name}...", task.Order, task.Name);
                await task.ExecuteAsync(cancellationToken);
                _logger.LogInformation("  [{Order}] {Name} completed", task.Order, task.Name);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Startup task '{Name}' failed", task.Name);
                _health.SetUnhealthy($"Startup task '{task.Name}' failed: {ex.Message}");
                throw;
            }
        }

        _health.SetHealthy();
        _logger.LogInformation("Globular service {Name} is ready", identity.Name);
    }
}
