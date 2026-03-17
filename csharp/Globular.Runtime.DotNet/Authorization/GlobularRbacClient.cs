// GlobularRbacClient — concrete RBAC gRPC client mirroring Go rbac_client.
// Connects to the Globular RBAC service for live authorization validation,
// role seeding, and resource ownership management.
//
// Go reference: golang/rbac/rbac_client/rbac_client.go
//
// All methods use real gRPC calls to the RBAC service.
// Fail-closed: if the RBAC service is unavailable, callers get an exception.

using Globular.Runtime.Authorization.Rbac;
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

    /// <summary>Local service token for RBAC calls (from /var/lib/globular/tokens/).</summary>
    public string? Token { get; set; }

    /// <summary>Service domain for metadata.</summary>
    public string Domain { get; set; } = "localhost";

    /// <summary>Service MAC address for metadata.</summary>
    public string? Mac { get; set; }

    /// <summary>Client TLS certificate path (for mTLS).</summary>
    public string? CertFile { get; set; }

    /// <summary>Client TLS key path (for mTLS).</summary>
    public string? KeyFile { get; set; }

    /// <summary>CA certificate path.</summary>
    public string? CaFile { get; set; }
}

/// <summary>
/// Concrete RBAC gRPC client. Implements both IRbacClient (for interceptor
/// authorization checks) and IRoleStore (for role seeding).
///
/// Mirrors the Go rbac_client behavior:
/// - ValidateAction with subject + action + resource infos
/// - GetRoleBinding for role-binding checks
/// - AddResourceOwner for post-creation ownership registration
/// - SetRoleBinding for role creation/seeding
/// - Token/domain injected via gRPC metadata (mirrors Go GetCtx())
/// - Timeout-guarded calls with fail-closed semantics
/// </summary>
public sealed class GlobularRbacClient : IRbacClient, IRoleStore, IDisposable
{
    private readonly RbacClientOptions _options;
    private readonly ILogger<GlobularRbacClient> _logger;
    private GrpcChannel? _channel;
    private RbacServiceClient? _client;

    public GlobularRbacClient(IOptions<RbacClientOptions> options, ILogger<GlobularRbacClient> logger)
    {
        _options = options.Value;
        _logger = logger;
    }

    // ── IRbacClient (interceptor authorization) ─────────────────────────

    /// <summary>
    /// Validates an action for a subject with optional resource path checks.
    /// Mirrors Go: rbacClient.ValidateAction(action, subject, subjectType, infos)
    /// Returns (allowed, denied) — fail-closed on error.
    /// </summary>
    public async Task<(bool Allowed, bool Denied)> ValidateActionAsync(
        string action, string subject, string subjectType,
        IReadOnlyList<ResourcePathCheck>? resourceChecks = null,
        CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var request = new ValidateActionRqst
        {
            Action = action,
            Subject = subject,
            Type = SubjectTypeExtensions.Parse(subjectType),
        };

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
        var response = await client.ValidateActionAsync(request, headers,
            cancellationToken: cts.Token);
        return (response.HasAccess, response.AccessDenied);
    }

    /// <summary>
    /// Checks if the subject has a role binding that grants the requested action.
    /// Mirrors Go: security.HasRolePermission(binding.GetRoles(), action)
    /// Returns false (deny) on any error — fail-closed.
    /// </summary>
    public async Task<bool> CheckRoleBindingAsync(string subject, string action,
        CancellationToken ct = default)
    {
        try
        {
            using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
            cts.CancelAfter(_options.Timeout);

            var client = await GetClientAsync(cts.Token);

            var request = new GetRoleBindingRqst { Subject = subject };
            var headers = BuildMetadata();
            var response = await client.GetRoleBindingAsync(request, headers,
                cancellationToken: cts.Token);

            // Check if any bound role grants the requested action.
            // Mirrors Go: iterate roles, check action match or wildcard.
            if (response?.Binding?.Roles is not null)
            {
                foreach (var role in response.Binding.Roles)
                {
                    if (role == action || role == "*" || role == "/*")
                        return true;
                }
            }

            return false;
        }
        catch (RpcException ex) when (ex.StatusCode is StatusCode.NotFound or StatusCode.Unavailable)
        {
            // No binding or RBAC unavailable -> deny (fail closed)
            return false;
        }
    }

    // ── IRoleStore (role seeding) ───────────────────────────────────────

    /// <summary>
    /// Checks if a role exists by querying the role binding for the role name.
    /// The RBAC service doesn't have a dedicated "role exists" RPC, so we use
    /// GetRoleBinding and check if the role is bound anywhere.
    ///
    /// For seeding purposes: if we can't confirm a role exists, we return false
    /// so the seeder will attempt to create it (SetRoleBinding is idempotent
    /// when the role already exists).
    /// </summary>
    public async Task<bool> RoleExistsAsync(string roleName, CancellationToken ct = default)
    {
        try
        {
            using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
            cts.CancelAfter(_options.Timeout);

            var client = await GetClientAsync(cts.Token);

            // Query role binding for the role name itself.
            // If a binding exists, the role has been configured.
            var request = new GetRoleBindingRqst { Subject = roleName };
            var headers = BuildMetadata();
            var response = await client.GetRoleBindingAsync(request, headers,
                cancellationToken: cts.Token);

            // Role exists if we get a non-null binding back (even with empty roles list).
            return response?.Binding is not null;
        }
        catch (RpcException ex) when (ex.StatusCode == StatusCode.NotFound)
        {
            // Role does not exist.
            return false;
        }
        catch (RpcException ex) when (ex.StatusCode == StatusCode.Unavailable)
        {
            _logger.LogWarning("RBAC service unavailable for role check: {Role}", roleName);
            // Conservative: return false so seeder will attempt creation
            // (SetRoleBinding is safe if role already exists).
            return false;
        }
    }

    /// <summary>
    /// Creates a role by setting a role binding with the given actions.
    /// Mirrors Go role creation via SetRoleBinding RPC.
    ///
    /// The RBAC service stores roles as bindings: the role name is the subject,
    /// and the actions are the roles list (which represent granted permissions).
    /// </summary>
    public async Task CreateRoleAsync(string roleName, IReadOnlyList<string> actions,
        IReadOnlyDictionary<string, string> metadata, CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var binding = new Rbac.RoleBinding
        {
            Subject = roleName,
            Roles = new List<string>(actions),
        };

        var request = new SetRoleBindingRqst { Binding = binding };
        var headers = BuildMetadata();

        await client.SetRoleBindingAsync(request, headers,
            cancellationToken: cts.Token);

        _logger.LogInformation("RBAC: created role {Role} with {Count} actions", roleName, actions.Count);
    }

    // ── Resource Ownership ──────────────────────────────────────────────

    /// <summary>
    /// Registers resource ownership after creation.
    /// Mirrors Go: rbacClient.AddResourceOwner(token, path, owner, resourceType, subjectType)
    /// </summary>
    public async Task AddResourceOwnerAsync(
        string token, string resourcePath, string owner,
        string resourceType, string subjectType,
        CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var request = new AddResourceOwnerRqst
        {
            Path = resourcePath,
            Subject = owner,
            ResourceType = resourceType,
            Type = SubjectTypeExtensions.Parse(subjectType),
        };

        // Use explicit token override for write operations (mirrors Go pattern).
        var headers = new Metadata { { "token", token } };
        if (!string.IsNullOrEmpty(_options.Domain))
            headers.Add("domain", _options.Domain);

        await client.AddResourceOwnerAsync(request, headers,
            cancellationToken: cts.Token);
    }

    /// <summary>
    /// Removes resource ownership.
    /// Mirrors Go: rbacClient.RemoveResourceOwner(token, path, owner, subjectType)
    /// </summary>
    public async Task RemoveResourceOwnerAsync(
        string token, string resourcePath, string owner,
        string subjectType,
        CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var request = new RemoveResourceOwnerRqst
        {
            Path = resourcePath,
            Subject = owner,
            Type = SubjectTypeExtensions.Parse(subjectType),
        };

        var headers = new Metadata { { "token", token } };
        if (!string.IsNullOrEmpty(_options.Domain))
            headers.Add("domain", _options.Domain);

        await client.RemoveResourceOwnerAsync(request, headers,
            cancellationToken: cts.Token);
    }

    /// <summary>
    /// Gets action resource infos for interceptor resource-level checks.
    /// Mirrors Go: rbacClient.GetActionResourceInfos(action)
    /// Results are cached by the caller (interceptor).
    /// </summary>
    public async Task<IReadOnlyList<ResourcePathCheck>> GetActionResourceInfosAsync(
        string action, CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var request = new GetActionResourceInfosRqst { Action = action };
        var headers = BuildMetadata();
        var response = await client.GetActionResourceInfosAsync(request, headers,
            cancellationToken: cts.Token);

        var result = new List<ResourcePathCheck>();
        if (response?.Infos is not null)
        {
            foreach (var info in response.Infos)
            {
                result.Add(new ResourcePathCheck(info.Path, info.Permission));
            }
        }

        return result;
    }

    /// <summary>
    /// Validates resource-level access.
    /// Mirrors Go: rbacClient.ValidateAccess(subject, subjectType, permission, path)
    /// Returns (hasAccess, accessDenied) — fail-closed on error.
    /// </summary>
    public async Task<(bool HasAccess, bool AccessDenied)> ValidateAccessAsync(
        string subject, string subjectType, string permission, string path,
        CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var request = new ValidateAccessRqst
        {
            Subject = subject,
            Type = SubjectTypeExtensions.Parse(subjectType),
            Permission = permission,
            Path = path,
        };

        var headers = BuildMetadata();
        var response = await client.ValidateAccessAsync(request, headers,
            cancellationToken: cts.Token);
        return (response.HasAccess, response.AccessDenied);
    }

    /// <summary>
    /// Removes all access for a subject (e.g., on account deletion).
    /// Mirrors Go: rbacClient.DeleteAllAccess(token, subject, subjectType)
    /// </summary>
    public async Task DeleteAllAccessAsync(
        string token, string subject, string subjectType,
        CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
        cts.CancelAfter(_options.Timeout);

        var client = await GetClientAsync(cts.Token);

        var request = new DeleteAllAccessRqst
        {
            Subject = subject,
            Type = SubjectTypeExtensions.Parse(subjectType),
        };

        var headers = new Metadata { { "token", token } };
        if (!string.IsNullOrEmpty(_options.Domain))
            headers.Add("domain", _options.Domain);

        await client.DeleteAllAccessAsync(request, headers,
            cancellationToken: cts.Token);
    }

    // ── Connection management ───────────────────────────────────────────

    private async Task<RbacServiceClient> GetClientAsync(CancellationToken ct)
    {
        if (_client is not null)
            return _client;

        // Create channel with retry — mirrors Go Reconnect() pattern.
        for (int i = 0; i < _options.MaxRetries; i++)
        {
            try
            {
                _channel = GrpcChannel.ForAddress(_options.Address);
                _client = new RbacServiceClient(_channel);
                return _client;
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

    /// <summary>
    /// Builds metadata with token + domain + mac.
    /// Mirrors Go: rbac_client.GetCtx() metadata injection.
    /// </summary>
    private Metadata BuildMetadata()
    {
        var md = new Metadata();
        if (!string.IsNullOrEmpty(_options.Token))
            md.Add("token", _options.Token);
        if (!string.IsNullOrEmpty(_options.Domain))
            md.Add("domain", _options.Domain);
        if (!string.IsNullOrEmpty(_options.Mac))
            md.Add("mac", _options.Mac);
        return md;
    }

    public void Dispose()
    {
        _channel?.Dispose();
    }
}
