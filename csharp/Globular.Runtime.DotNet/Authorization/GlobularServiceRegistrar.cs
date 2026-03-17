// GlobularServiceRegistrar — real service registration implementation.
// Publishes effective runtime state to the Globular management plane
// so the platform can transparently manage the service.
//
// Replaces NoOpGlobularDiscoveryRegistrar for managed deployments.

using System.Text.Json;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Interface for publishing service state to the cluster management plane.
/// Implementations may write to etcd, call a Globular API, or both.
/// </summary>
public interface IServiceStatePublisher
{
    /// <summary>Publish the effective runtime state for this service instance.</summary>
    Task PublishAsync(EffectiveRuntimeState state, CancellationToken ct = default);

    /// <summary>Remove the registration on shutdown.</summary>
    Task UnpublishAsync(string serviceId, CancellationToken ct = default);
}

/// <summary>
/// Real service registrar that publishes effective runtime state to the
/// Globular management plane. Implements IGlobularDiscoveryRegistrar for
/// integration with the existing startup pipeline.
/// </summary>
public sealed class GlobularServiceRegistrar : IGlobularDiscoveryRegistrar
{
    private readonly IServiceStatePublisher _publisher;
    private readonly EffectiveRuntimeState _state;
    private readonly ILogger<GlobularServiceRegistrar> _logger;

    public GlobularServiceRegistrar(
        IServiceStatePublisher publisher,
        EffectiveRuntimeState state,
        ILogger<GlobularServiceRegistrar> logger)
    {
        _publisher = publisher;
        _state = state;
        _logger = logger;
    }

    public async Task RegisterAsync(CancellationToken cancellationToken = default)
    {
        _state.LastHeartbeat = DateTime.UtcNow;
        _state.Readiness = "ready";

        try
        {
            await _publisher.PublishAsync(_state, cancellationToken);
            _logger.LogInformation(
                "Service registered: {Service} at {Address}:{Port} (authz={Mode}, rbac={Rbac}, manifests={Manifests})",
                _state.ServiceName, _state.GrpcAddress, _state.GrpcPort,
                _state.AuthzMode, _state.RbacActive, _state.ManifestsLoaded);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to register service {Service}", _state.ServiceName);
        }
    }

    public async Task UnregisterAsync(CancellationToken cancellationToken = default)
    {
        try
        {
            await _publisher.UnpublishAsync(_state.ServiceId, cancellationToken);
            _logger.LogInformation("Service unregistered: {Service}", _state.ServiceName);
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Failed to unregister service {Service}", _state.ServiceName);
        }
    }
}

/// <summary>
/// Logs effective state to structured logging when no real publisher is configured.
/// Useful for development and as a fallback. NOT a replacement for real registration.
/// </summary>
public sealed class LoggingServiceStatePublisher : IServiceStatePublisher
{
    private readonly ILogger<LoggingServiceStatePublisher> _logger;

    public LoggingServiceStatePublisher(ILogger<LoggingServiceStatePublisher> logger)
    {
        _logger = logger;
    }

    public Task PublishAsync(EffectiveRuntimeState state, CancellationToken ct = default)
    {
        var json = JsonSerializer.Serialize(state, new JsonSerializerOptions { WriteIndented = true });
        _logger.LogInformation("Effective runtime state:\n{State}", json);
        return Task.CompletedTask;
    }

    public Task UnpublishAsync(string serviceId, CancellationToken ct = default)
    {
        _logger.LogInformation("Service state unpublished: {ServiceId}", serviceId);
        return Task.CompletedTask;
    }
}
