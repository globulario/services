// EtcdServiceStatePublisher — publishes service runtime state to etcd
// for transparent Globular management. This is the real publisher that
// replaces the logging fallback in managed/cluster deployments.
//
// Uses the etcd key pattern: /globular/services/<serviceId>/runtime-state
// This aligns with how Go services register their state in etcd.

using System.Text.Json;
using Microsoft.Extensions.Logging;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Interface for raw etcd key-value operations.
/// Implemented by the actual etcd client wrapper.
/// </summary>
public interface IEtcdClient
{
    Task PutAsync(string key, string value, CancellationToken ct = default);
    Task DeleteAsync(string key, CancellationToken ct = default);
}

/// <summary>
/// Publishes EffectiveRuntimeState to etcd under the standard Globular
/// service registration key path. This makes the service's effective
/// configuration transparently inspectable by Globular Admin, CLI, and
/// cluster management tooling.
///
/// Key format: /globular/services/{serviceId}/runtime-state
/// </summary>
public sealed class EtcdServiceStatePublisher : IServiceStatePublisher
{
    private readonly IEtcdClient _etcd;
    private readonly ILogger<EtcdServiceStatePublisher> _logger;

    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        WriteIndented = false,
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
    };

    public EtcdServiceStatePublisher(IEtcdClient etcd, ILogger<EtcdServiceStatePublisher> logger)
    {
        _etcd = etcd;
        _logger = logger;
    }

    public async Task PublishAsync(EffectiveRuntimeState state, CancellationToken ct = default)
    {
        var key = $"/globular/services/{state.ServiceId}/runtime-state";
        var value = JsonSerializer.Serialize(state, JsonOpts);

        await _etcd.PutAsync(key, value, ct);
        _logger.LogInformation("Published runtime state to etcd: {Key}", key);
    }

    public async Task UnpublishAsync(string serviceId, CancellationToken ct = default)
    {
        var key = $"/globular/services/{serviceId}/runtime-state";
        await _etcd.DeleteAsync(key, ct);
        _logger.LogInformation("Removed runtime state from etcd: {Key}", key);
    }
}
