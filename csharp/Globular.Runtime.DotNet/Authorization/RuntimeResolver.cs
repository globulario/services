// RuntimeResolver — resolves effective service identity, ports, and domain
// at startup. Mirrors the Go service startup contract where defaults are
// computed, ports allocated, and identity generated if missing.

using System.Net;
using System.Net.Sockets;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Port allocation strategy. Implementations may use OS-level allocation,
/// configuration, or a Globular port allocator service.
/// </summary>
public interface IPortAllocator
{
    /// <summary>Allocate an available port. Returns 0 if allocation fails.</summary>
    int AllocatePort(int preferredPort = 0);
}

/// <summary>
/// Default port allocator: uses the configured port, or finds an available one.
/// </summary>
public sealed class DefaultPortAllocator : IPortAllocator
{
    public int AllocatePort(int preferredPort = 0)
    {
        if (preferredPort > 0)
            return preferredPort;

        // Find an available port by binding to port 0.
        using var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        var port = ((IPEndPoint)listener.LocalEndpoint).Port;
        listener.Stop();
        return port;
    }
}

/// <summary>
/// Resolves the effective runtime configuration at startup.
/// Populates an EffectiveRuntimeState with resolved identity, ports,
/// domain, and authz configuration.
/// </summary>
public sealed class RuntimeResolver
{
    private readonly IPortAllocator _portAllocator;
    private readonly ILogger<RuntimeResolver> _logger;

    public RuntimeResolver(IPortAllocator portAllocator, ILogger<RuntimeResolver> logger)
    {
        _portAllocator = portAllocator;
        _logger = logger;
    }

    /// <summary>
    /// Resolve effective runtime state from options + environment + defaults.
    /// </summary>
    public EffectiveRuntimeState Resolve(
        GlobularHostOptions options,
        AuthorizationMode authzMode = AuthorizationMode.RbacStrict)
    {
        var state = new EffectiveRuntimeState();
        var identity = options.Service;
        var network = options.Network;

        // Identity
        state.ServiceName = identity.Name;
        state.ServiceId = !string.IsNullOrEmpty(identity.ServiceId)
            ? identity.ServiceId
            : GenerateServiceId(identity.Name);
        state.Version = identity.Version;
        state.PublisherId = identity.PublisherId;
        state.Domain = identity.Domain;
        state.ProtoPackage = identity.ProtoPackage;
        state.ProtoService = identity.ProtoService;

        // Node ID from environment if available
        state.NodeId = Environment.GetEnvironmentVariable("GLOBULAR_NODE_ID");

        // Domain resolution: env override → config → default
        if (string.IsNullOrEmpty(state.Domain))
            state.Domain = Environment.GetEnvironmentVariable("GLOBULAR_DOMAIN") ?? "localhost";

        // Port resolution
        state.GrpcPort = _portAllocator.AllocatePort(network.GrpcPort);
        state.GrpcAddress = !string.IsNullOrEmpty(network.PublicAddress)
            ? network.PublicAddress
            : ResolveAddress(network.GrpcBindAddress, state.GrpcPort);
        state.TlsEnabled = network.TlsEnabled;
        state.Protocol = network.TlsEnabled ? "grpcs" : "grpc";

        // Authorization mode
        state.AuthzMode = authzMode.ToString();

        _logger.LogInformation(
            "Runtime resolved: {Service} id={Id} at {Address}:{Port} domain={Domain} authz={Mode}",
            state.ServiceName, state.ServiceId, state.GrpcAddress,
            state.GrpcPort, state.Domain, state.AuthzMode);

        return state;
    }

    private static string GenerateServiceId(string serviceName)
    {
        // Stable ID from hostname + service name + process ID.
        var host = Environment.MachineName;
        var pid = Environment.ProcessId;
        return $"{serviceName}:{host}:{pid}";
    }

    private static string ResolveAddress(string bindAddress, int port)
    {
        // If bound to 0.0.0.0, resolve to the machine's hostname.
        if (bindAddress == "0.0.0.0" || bindAddress == "::")
            return $"{Dns.GetHostName()}:{port}";
        return $"{bindAddress}:{port}";
    }
}
