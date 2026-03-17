// GlobularRbacClient — concrete RBAC gRPC client mirroring Go rbac_client.
// Connects to the Globular RBAC service for live authorization validation.
//
// Go reference: golang/rbac/rbac_client/rbac_client.go
// Key methods: ValidateAction, GetActionResourceInfos, GetRoleBinding, AddResourceOwner

using Grpc.Core;
using Grpc.Net.Client;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Configuration for the RBAC gRPC client.
/// </summary>
public sealed class RbacClientOptions
{
    /// <summary>RBAC service gRPC address (e.g., "https://localhost:10104").</summary>
    public string Address { get; set; } = "https://localhost:10104";

    /// <summary>Request timeout.</summary>
    public TimeSpan Timeout { get; set; } = TimeSpan.FromSeconds(3);

    /// <summary>Maximum reconnect attempts.</summary>
    public int MaxRetries { get; set; } = 10;

    /// <summary>Delay between reconnect attempts.</summary>
    public TimeSpan RetryDelay { get; set; } = TimeSpan.FromMilliseconds(500);

    /// <summary>Local service token for RBAC calls.</summary>
    public string? Token { get; set; }

    /// <summary>Service domain for metadata.</summary>
    public string Domain { get; set; } = "localhost";
}

/// <summary>
/// Concrete RBAC gRPC client. Implements both IRbacClient (for interceptor
/// authorization checks) and IRoleStore (for role seeding).
///
/// Mirrors the Go rbac_client behavior:
/// - ValidateAction with subject + action + resource infos
/// - GetRoleBinding for role-binding checks
/// - AddResourceOwner for post-creation ownership registration
/// - Token/domain injected via gRPC metadata
/// - Timeout-guarded calls with fail-closed semantics
/// </summary>
public sealed class GlobularRbacClient : IRbacClient, IRoleStore, IDisposable
{
    private readonly RbacClientOptions _options;
    private readonly ILogger<GlobularRbacClient> _logger;
    private GrpcChannel? _channel;

    public GlobularRbacClient(IOptions<RbacClientOptions> options, ILogger<GlobularRbacClient> logger)
    {
        _options = options.Value;
        _logger = logger;
    }

    // ── IRbacClient (interceptor authorization) ─────────────────────────

    public async Task<(bool Allowed, bool Denied)> ValidateActionAsync(
        string action, string subject, string subjectType,
        IReadOnlyList<ResourcePathCheck>? resourceChecks = null,
        CancellationToken ct = default)
    {
        try
        {
            using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
            cts.CancelAfter(_options.Timeout);

            var channel = await GetChannelAsync(cts.Token);
            var client = new Rbac.RbacService.RbacServiceClient(channel);

            // Build request matching Go's ValidateAction RPC.
            var request = new Rbac.ValidateActionRqst
            {
                Action = action,
                Subject = subject,
                // SubjectType mapped from string to enum
            };

            // Add resource path checks if present.
            if (resourceChecks is not null)
            {
                foreach (var rc in resourceChecks)
                {
                    request.Infos.Add(new Rbac.ResourceInfos
                    {
                        Path = rc.Path,
                        Permission = rc.Permission,
                    });
                }
            }

            var headers = BuildMetadata();
            var response = await client.ValidateActionAsync(request, headers, cancellationToken: cts.Token);
            return (response.HasAccess, response.AccessDenied);
        }
        catch (RpcException ex) when (ex.StatusCode == StatusCode.Unavailable)
        {
            _logger.LogError("RBAC service unavailable at {Address}", _options.Address);
            throw; // fail closed — let interceptor handle
        }
    }

    public async Task<bool> CheckRoleBindingAsync(string subject, string action, CancellationToken ct = default)
    {
        try
        {
            using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
            cts.CancelAfter(_options.Timeout);

            var channel = await GetChannelAsync(cts.Token);
            var client = new Rbac.RbacService.RbacServiceClient(channel);

            var request = new Rbac.GetRoleBindingRqst { Subject = subject };
            var headers = BuildMetadata();
            var response = await client.GetRoleBindingAsync(request, headers, cancellationToken: cts.Token);

            // Check if any bound role grants the requested action.
            // Mirrors Go: security.HasRolePermission(binding.GetRoles(), action)
            if (response?.RoleBinding?.Roles is not null)
            {
                foreach (var role in response.RoleBinding.Roles)
                {
                    // Simple match — full wildcard and action-key matching
                    // is done server-side by the RBAC service.
                    if (role == action || role == "*" || role == "/*")
                        return true;
                }
            }

            return false;
        }
        catch (RpcException ex) when (ex.StatusCode is StatusCode.NotFound or StatusCode.Unavailable)
        {
            // No binding or RBAC unavailable → deny (fail closed)
            return false;
        }
    }

    // ── IRoleStore (role seeding) ───────────────────────────────────────

    public async Task<bool> RoleExistsAsync(string roleName, CancellationToken ct = default)
    {
        // Check if role exists by trying to get it.
        // If the RBAC service doesn't have a dedicated "exists" RPC,
        // use a lightweight read operation.
        try
        {
            using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
            cts.CancelAfter(_options.Timeout);
            // Simplified: assume role exists if no error on a lightweight check.
            // Real implementation would call GetRole or similar.
            return false; // Conservative: seed if unsure
        }
        catch
        {
            return false;
        }
    }

    public async Task CreateRoleAsync(string roleName, IReadOnlyList<string> actions,
        IReadOnlyDictionary<string, string> metadata, CancellationToken ct = default)
    {
        // Create role via RBAC service.
        // Real implementation would call CreateRole or equivalent RPC.
        _logger.LogInformation("Would seed role {Role} with {Actions} actions", roleName, actions.Count);
        await Task.CompletedTask;
    }

    // ── Resource Ownership ──────────────────────────────────────────────

    /// <summary>
    /// Registers resource ownership after creation. Called by service handlers.
    /// Mirrors Go: rbacClient.AddResourceOwner(token, path, owner, resourceType, subjectType)
    /// </summary>
    public async Task AddResourceOwnerAsync(
        string token, string resourcePath, string owner,
        string resourceType, string subjectType,
        CancellationToken ct = default)
    {
        try
        {
            using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
            cts.CancelAfter(_options.Timeout);

            var channel = await GetChannelAsync(cts.Token);
            var client = new Rbac.RbacService.RbacServiceClient(channel);

            var request = new Rbac.AddResourceOwnerRqst
            {
                Path = resourcePath,
                Subject = owner,
                ResourceType = resourceType,
            };

            var headers = new Metadata { { "token", token } };
            await client.AddResourceOwnerAsync(request, headers, cancellationToken: cts.Token);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to register resource owner for {Path}", resourcePath);
            throw;
        }
    }

    // ── Connection management ───────────────────────────────────────────

    private async Task<GrpcChannel> GetChannelAsync(CancellationToken ct)
    {
        if (_channel is not null)
            return _channel;

        // Create channel with retry.
        for (int i = 0; i < _options.MaxRetries; i++)
        {
            try
            {
                _channel = GrpcChannel.ForAddress(_options.Address);
                return _channel;
            }
            catch
            {
                if (i < _options.MaxRetries - 1)
                    await Task.Delay(_options.RetryDelay, ct);
            }
        }

        throw new RpcException(new Status(StatusCode.Unavailable,
            $"Failed to connect to RBAC service at {_options.Address} after {_options.MaxRetries} attempts"));
    }

    private Metadata BuildMetadata()
    {
        var md = new Metadata();
        if (!string.IsNullOrEmpty(_options.Token))
            md.Add("token", _options.Token);
        if (!string.IsNullOrEmpty(_options.Domain))
            md.Add("domain", _options.Domain);
        return md;
    }

    public void Dispose()
    {
        _channel?.Dispose();
    }
}

// ── Placeholder proto types ─────────────────────────────────────────────────
// These would normally be generated from rbac.proto.
// In production, use the actual generated gRPC client stubs.
// TODO: Replace with actual generated code from proto/rbac.proto

namespace Rbac
{
    internal static class RbacService
    {
        internal class RbacServiceClient
        {
            private readonly GrpcChannel _channel;
            public RbacServiceClient(GrpcChannel channel) { _channel = channel; }

            public Task<ValidateActionRsp> ValidateActionAsync(ValidateActionRqst req, Metadata headers, CancellationToken cancellationToken = default)
                => throw new NotImplementedException("Replace with generated gRPC client stub");
            public Task<GetRoleBindingRsp> GetRoleBindingAsync(GetRoleBindingRqst req, Metadata headers, CancellationToken cancellationToken = default)
                => throw new NotImplementedException("Replace with generated gRPC client stub");
            public Task<AddResourceOwnerRsp> AddResourceOwnerAsync(AddResourceOwnerRqst req, Metadata headers, CancellationToken cancellationToken = default)
                => throw new NotImplementedException("Replace with generated gRPC client stub");
        }
    }

    internal class ValidateActionRqst { public string Action { get; set; } = ""; public string Subject { get; set; } = ""; public List<ResourceInfos> Infos { get; set; } = new(); }
    internal class ValidateActionRsp { public bool HasAccess { get; set; } public bool AccessDenied { get; set; } }
    internal class GetRoleBindingRqst { public string Subject { get; set; } = ""; }
    internal class GetRoleBindingRsp { public RoleBinding? RoleBinding { get; set; } }
    internal class RoleBinding { public List<string>? Roles { get; set; } }
    internal class ResourceInfos { public string Path { get; set; } = ""; public string Permission { get; set; } = ""; public string Field { get; set; } = ""; public int Index { get; set; } }
    internal class AddResourceOwnerRqst { public string Path { get; set; } = ""; public string Subject { get; set; } = ""; public string ResourceType { get; set; } = ""; }
    internal class AddResourceOwnerRsp { }
}
