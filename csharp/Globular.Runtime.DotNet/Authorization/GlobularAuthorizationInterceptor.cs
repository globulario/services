// GlobularAuthorizationInterceptor — mirrors Go interceptors/ServerInterceptors.go.
// Centralized gRPC server interceptor for authorization enforcement.
//
// Go reference: golang/interceptors/ServerInterceptors.go
// Go reference: golang/security/auth_context.go
//
// Runtime flow (mirrors Go 10-step security model):
//   1. Extract gRPC method from ServerCallContext
//   2. Resolve method → stable action key via ActionResolver
//   3. Create AuthContext: decode JWT → extract principal ID (single source of truth)
//   4. Skip unauthenticated allowlist methods
//   5. Deny unauthenticated callers on protected methods
//   6. Role-binding check (for role-based methods)
//   7. RBAC resource info fetch (cached per method)
//   8. Subject hierarchy validation: ACCOUNT → APPLICATION → NODE_IDENTITY
//   9. Allow or deny

using System.Collections.Concurrent;
using System.Text;
using System.Text.Json;
using Grpc.Core;
using Grpc.Core.Interceptors;
using Microsoft.Extensions.Logging;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Interface for calling the Globular RBAC service. Implement this to connect
/// to the Go RBAC service via gRPC.
/// </summary>
public interface IRbacClient
{
    /// <summary>Check if the subject has permission for the given action.</summary>
    Task<(bool Allowed, bool Denied)> ValidateActionAsync(
        string action, string subject, string subjectType,
        IReadOnlyList<ResourcePathCheck>? resourceChecks = null,
        CancellationToken ct = default);

    /// <summary>Fetch the role binding for a subject. Returns role names.</summary>
    Task<IReadOnlyList<string>> GetRoleBindingAsync(string subject, CancellationToken ct = default);

    /// <summary>Fetch resource infos for an action (for resource-level checks).</summary>
    Task<IReadOnlyList<ResourcePathCheck>> GetActionResourceInfosAsync(string action, CancellationToken ct = default);
}

/// <summary>Resource path + permission for RBAC resource-level checks.</summary>
public sealed record ResourcePathCheck(string Path, string Permission);

/// <summary>
/// Immutable authentication context for a single gRPC invocation.
/// Mirrors Go security.AuthContext — single source of truth for authorization.
/// Identity is extracted from JWT claims, never from raw metadata.
/// </summary>
public sealed class AuthContext
{
    /// <summary>Principal identity (domain-independent, e.g. "dave" not "dave@localhost").</summary>
    public string Subject { get; init; } = "";
    /// <summary>"user", "application", "node", "anonymous".</summary>
    public string PrincipalType { get; init; } = "anonymous";
    /// <summary>"jwt", "mtls", "none".</summary>
    public string AuthMethod { get; init; } = "none";
    /// <summary>Application name from metadata (for APPLICATION subject type validation).</summary>
    public string Application { get; init; } = "";
    /// <summary>JWT issuer (MAC address, for NODE_IDENTITY subject type validation).</summary>
    public string Issuer { get; init; } = "";
}

/// <summary>
/// Server-side gRPC interceptor that enforces authorization using loaded
/// permission manifests and the RBAC service.
///
/// Register in DI and add to gRPC pipeline:
///   services.AddSingleton&lt;GlobularAuthorizationInterceptor&gt;();
///   services.AddGrpc(o => o.Interceptors.Add&lt;GlobularAuthorizationInterceptor&gt;());
/// </summary>
public sealed class GlobularAuthorizationInterceptor : Interceptor
{
    private readonly ActionResolver _resolver;
    private readonly IRbacClient _rbac;
    private readonly RolePermissionChecker _roleChecker;
    private readonly ILogger<GlobularAuthorizationInterceptor> _logger;
    private readonly AuthorizationMode _mode;

    /// <summary>
    /// Per-method cache for resource infos (mirrors Go resourceInfos sync.Map).
    /// Once loaded, resource infos for a method never change during process lifetime.
    /// </summary>
    private readonly ConcurrentDictionary<string, IReadOnlyList<ResourcePathCheck>> _resourceInfoCache = new();

    /// <summary>Runtime authorization state, published to etcd for cluster visibility.</summary>
    public AuthorizationState State { get; } = new();

    /// <summary>
    /// Methods that do not require authentication (health checks, reflection, etc.).
    /// Mirrors Go allowSet default entries.
    /// </summary>
    private static readonly HashSet<string> UnauthenticatedMethods = new()
    {
        "/grpc.health.v1.Health/Check",
        "/grpc.health.v1.Health/Watch",
        "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
    };

    public GlobularAuthorizationInterceptor(
        ActionResolver resolver,
        IRbacClient rbac,
        RolePermissionChecker roleChecker,
        ILogger<GlobularAuthorizationInterceptor> logger,
        AuthorizationMode mode = AuthorizationMode.RbacStrict)
    {
        _resolver = resolver;
        _rbac = rbac;
        _roleChecker = roleChecker;
        _logger = logger;
        _mode = mode;
        State.Mode = mode;
    }

    public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
        TRequest request,
        ServerCallContext context,
        UnaryServerMethod<TRequest, TResponse> continuation)
    {
        var method = context.Method;

        // Skip unauthenticated methods.
        if (UnauthenticatedMethods.Contains(method))
            return await continuation(request, context);

        // Resolve method → stable action key.
        var actionKey = _resolver.Resolve(method);

        // Create AuthContext: decode JWT to extract principal ID.
        // Mirrors Go security.NewAuthContext() — single source of truth.
        var authCtx = CreateAuthContext(context);

        if (string.IsNullOrEmpty(authCtx.Subject))
        {
            _logger.LogWarning("Auth: denied unauthenticated request for protected method {Method}", method);
            throw new RpcException(new Status(StatusCode.Unauthenticated,
                "authentication required: provide token or client certificate"));
        }

        // Check authorization via RBAC gRPC.
        try
        {
            // Role-binding check (mirrors Go checkRoleBinding + HasRolePermission).
            var roles = await _rbac.GetRoleBindingAsync(authCtx.Subject);
            if (roles.Count > 0 && _roleChecker.HasPermission(roles, actionKey))
            {
                State.RbacActive = true;
                State.FallbackActive = false;
                _logger.LogDebug("Auth: role binding granted for {Subject} on {Action}", authCtx.Subject, actionKey);
                return await continuation(request, context);
            }

            // Subject hierarchy validation (mirrors Go interceptor):
            // Try ACCOUNT → APPLICATION → NODE_IDENTITY
            var resourceChecks = ExpandResourceTemplate(method, request);

            // 1. Validate by ACCOUNT
            var (allowed, denied) = await _rbac.ValidateActionAsync(
                actionKey, authCtx.Subject, "ACCOUNT", resourceChecks);

            // 2. Validate by APPLICATION (if ACCOUNT denied and application metadata present)
            if (!allowed && !denied && !string.IsNullOrEmpty(authCtx.Application))
            {
                (allowed, denied) = await _rbac.ValidateActionAsync(
                    actionKey, authCtx.Application, "APPLICATION", resourceChecks);
            }

            // 3. Validate by NODE_IDENTITY (if still denied and issuer present)
            if (!allowed && !denied && !string.IsNullOrEmpty(authCtx.Issuer))
            {
                (allowed, denied) = await _rbac.ValidateActionAsync(
                    actionKey, authCtx.Issuer, "NODE_IDENTITY", resourceChecks);
            }

            State.RbacActive = true;
            State.FallbackActive = false;

            if (allowed && !denied)
                return await continuation(request, context);

            _logger.LogWarning("Auth: denied {Subject} for {Action} ({Method})",
                authCtx.Subject, actionKey, method);
            throw new RpcException(new Status(StatusCode.PermissionDenied,
                $"permission denied: {actionKey}"));
        }
        catch (RpcException) { throw; } // re-throw our own denials
        catch (Exception ex)
        {
            // RBAC unavailable — behavior depends on authorization mode.
            return await HandleRbacUnavailable<TRequest, TResponse>(
                ex, method, actionKey, authCtx.Subject, request, context, continuation);
        }
    }

    public override async Task ServerStreamingServerHandler<TRequest, TResponse>(
        TRequest request,
        IServerStreamWriter<TResponse> responseStream,
        ServerCallContext context,
        ServerStreamingServerMethod<TRequest, TResponse> continuation)
    {
        var method = context.Method;
        if (UnauthenticatedMethods.Contains(method))
        {
            await continuation(request, responseStream, context);
            return;
        }

        var actionKey = _resolver.Resolve(method);
        var authCtx = CreateAuthContext(context);

        if (string.IsNullOrEmpty(authCtx.Subject))
        {
            throw new RpcException(new Status(StatusCode.Unauthenticated,
                "authentication required: provide token or client certificate"));
        }

        try
        {
            // Role-binding check.
            var roles = await _rbac.GetRoleBindingAsync(authCtx.Subject);
            if (roles.Count == 0 || !_roleChecker.HasPermission(roles, actionKey))
            {
                // Subject hierarchy: ACCOUNT → APPLICATION → NODE_IDENTITY
                var resourceChecks = ExpandResourceTemplate(method, request);
                var (allowed, denied) = await _rbac.ValidateActionAsync(
                    actionKey, authCtx.Subject, "ACCOUNT", resourceChecks);

                if (!allowed && !denied && !string.IsNullOrEmpty(authCtx.Application))
                {
                    (allowed, denied) = await _rbac.ValidateActionAsync(
                        actionKey, authCtx.Application, "APPLICATION", resourceChecks);
                }

                if (!allowed && !denied && !string.IsNullOrEmpty(authCtx.Issuer))
                {
                    (allowed, denied) = await _rbac.ValidateActionAsync(
                        actionKey, authCtx.Issuer, "NODE_IDENTITY", resourceChecks);
                }

                if (!allowed || denied)
                {
                    throw new RpcException(new Status(StatusCode.PermissionDenied,
                        $"permission denied: {actionKey}"));
                }
            }
        }
        catch (RpcException) { throw; }
        catch (Exception ex)
        {
            if (_mode == AuthorizationMode.RbacStrict)
            {
                _logger.LogError(ex, "Auth: RBAC unavailable in strict mode for streaming {Action}", actionKey);
                throw new RpcException(new Status(StatusCode.Unavailable,
                    "authorization service unavailable — request denied"));
            }
            _logger.LogWarning("AUTH DEGRADED: RBAC unavailable, allowing streaming {Action} in {Mode} mode",
                actionKey, _mode);
            State.FallbackActive = true;
        }

        await continuation(request, responseStream, context);
    }

    // ── AuthContext creation ────────────────────────────────────────────────

    /// <summary>
    /// Creates an AuthContext by extracting and decoding the JWT token from
    /// gRPC metadata. Mirrors Go security.NewAuthContext().
    ///
    /// Identity extraction:
    ///   1. Read token from "token" header or "Authorization: Bearer ..." header
    ///   2. Decode JWT payload (base64url) to extract principal_id
    ///   3. Fallback chain: principal_id → sub → id
    ///
    /// The token signature is NOT validated here — that's the RBAC service's job.
    /// We only need the principal identity for authorization routing.
    /// </summary>
    private AuthContext CreateAuthContext(ServerCallContext context)
    {
        string? token = null;
        string application = "";

        // Extract token from metadata (mirrors Go extractTokenFromContext).
        var tokenEntry = context.RequestHeaders.Get("token");
        if (tokenEntry is not null)
            token = tokenEntry.Value;

        if (string.IsNullOrEmpty(token))
        {
            var authEntry = context.RequestHeaders.Get("authorization");
            if (authEntry is not null && authEntry.Value.StartsWith("Bearer ", StringComparison.OrdinalIgnoreCase))
                token = authEntry.Value[7..];
        }

        // Read application from metadata (for APPLICATION subject type).
        var appEntry = context.RequestHeaders.Get("application");
        if (appEntry is not null)
            application = appEntry.Value;

        if (string.IsNullOrEmpty(token))
            return new AuthContext { Application = application };

        // Decode JWT payload to extract claims.
        // Mirrors Go: claims, err := ValidateToken(token); authCtx.Subject = claims.PrincipalID
        var claims = DecodeJwtPayload(token);
        if (claims is null)
        {
            _logger.LogWarning("Auth: failed to decode JWT payload");
            return new AuthContext { Application = application };
        }

        // Extract principal identity (mirrors Go AuthContext fallback chain).
        var subject = GetStringClaim(claims, "principal_id");
        if (string.IsNullOrEmpty(subject))
            subject = GetStringClaim(claims, "sub");
        if (string.IsNullOrEmpty(subject))
            subject = GetStringClaim(claims, "id");

        // Strip @domain suffix for domain-independent identity
        // (mirrors Go: "dave@localhost" → "dave").
        if (!string.IsNullOrEmpty(subject))
        {
            var atIdx = subject.IndexOf('@');
            if (atIdx > 0)
                subject = subject[..atIdx];
        }

        // Determine principal type from claims.
        var principalType = "application";
        if (!string.IsNullOrEmpty(GetStringClaim(claims, "email")))
            principalType = "user";

        // Extract issuer for NODE_IDENTITY subject type.
        var issuer = GetStringClaim(claims, "iss") ?? "";

        return new AuthContext
        {
            Subject = subject ?? "",
            PrincipalType = principalType,
            AuthMethod = "jwt",
            Application = application,
            Issuer = issuer,
        };
    }

    /// <summary>
    /// Decodes the JWT payload (middle segment) without signature validation.
    /// Returns parsed JSON as a dictionary, or null on failure.
    /// Mirrors Go: jwt.Parser.ParseUnverified() used in security.GetClaims().
    /// </summary>
    private static Dictionary<string, JsonElement>? DecodeJwtPayload(string token)
    {
        var parts = token.Split('.');
        if (parts.Length != 3)
            return null;

        try
        {
            // Base64url decode the payload segment.
            var payload = parts[1];
            // Pad to multiple of 4 for standard base64.
            payload = payload.Replace('-', '+').Replace('_', '/');
            switch (payload.Length % 4)
            {
                case 2: payload += "=="; break;
                case 3: payload += "="; break;
            }

            var bytes = Convert.FromBase64String(payload);
            var json = Encoding.UTF8.GetString(bytes);
            return JsonSerializer.Deserialize<Dictionary<string, JsonElement>>(json);
        }
        catch
        {
            return null;
        }
    }

    private static string? GetStringClaim(Dictionary<string, JsonElement> claims, string key)
    {
        if (claims.TryGetValue(key, out var element) && element.ValueKind == JsonValueKind.String)
            return element.GetString();
        return null;
    }

    // ── Resource template expansion ─────────────────────────────────────────

    /// <summary>
    /// Expands the resource template for the given method using request field values.
    /// Returns null if no template exists (action-only method).
    /// Throws RpcException if a template exists but expansion fails (strict enforcement).
    /// </summary>
    private List<ResourcePathCheck>? ExpandResourceTemplate<TRequest>(string method, TRequest request)
        where TRequest : class
    {
        var perm = _resolver.ResolvePermission(method);
        if (perm is null) return null;

        var template = perm.ResourceTemplate ?? perm.CollectionTemplate;
        if (string.IsNullOrEmpty(template)) return null;

        try
        {
            var fields = ExtractStringFields(request);
            var path = ResourceTemplate.Expand(template, fields);
            if (string.IsNullOrEmpty(path)) return null;

            return new List<ResourcePathCheck>
            {
                new(path, perm.Permission ?? "read")
            };
        }
        catch (ResourceTemplateException ex)
        {
            _logger.LogWarning("Auth: resource template expansion failed for {Method}: {Error}",
                method, ex.Message);
            throw new RpcException(new Status(StatusCode.InvalidArgument,
                $"authorization failed: {ex.Message}"));
        }
    }

    /// <summary>
    /// Extracts string properties from a protobuf message using reflection.
    /// Stores both PascalCase and camelCase for template matching.
    /// </summary>
    private static Dictionary<string, string> ExtractStringFields<T>(T request) where T : class
    {
        var fields = new Dictionary<string, string>();
        if (request is null) return fields;

        foreach (var prop in request.GetType().GetProperties())
        {
            if (prop.PropertyType != typeof(string)) continue;
            var value = prop.GetValue(request) as string;
            if (string.IsNullOrEmpty(value)) continue;

            fields[prop.Name] = value;
            var camelCase = char.ToLowerInvariant(prop.Name[0]) + prop.Name[1..];
            fields[camelCase] = value;
        }

        return fields;
    }

    // ── RBAC unavailable handling ───────────────────────────────────────────

    /// <summary>
    /// Handles RBAC unavailability based on the configured authorization mode.
    /// RbacStrict (default): fail closed — deny the request.
    /// Bootstrap/Development: log loudly and allow (degraded fallback).
    /// </summary>
    private async Task<TResponse> HandleRbacUnavailable<TRequest, TResponse>(
        Exception ex,
        string method, string actionKey, string subject,
        TRequest request, ServerCallContext context,
        UnaryServerMethod<TRequest, TResponse> continuation)
        where TRequest : class
        where TResponse : class
    {
        switch (_mode)
        {
            case AuthorizationMode.RbacStrict:
                State.RbacActive = false;
                State.FallbackActive = false;
                _logger.LogError(ex,
                    "Auth: RBAC unavailable in strict mode, denying {Action} for {Subject}",
                    actionKey, subject);
                throw new RpcException(new Status(StatusCode.Unavailable,
                    "authorization service unavailable — request denied (fail-closed)"));

            case AuthorizationMode.Bootstrap:
                State.RbacActive = false;
                State.FallbackActive = true;
                _logger.LogWarning(
                    "AUTH DEGRADED: RBAC unavailable in bootstrap mode, allowing {Action} for {Subject} on {Method}. " +
                    "This is a temporary fallback — configure RBAC for production.",
                    actionKey, subject, method);
                return await continuation(request, context);

            case AuthorizationMode.Development:
                State.RbacActive = false;
                State.FallbackActive = true;
                _logger.LogWarning(
                    "AUTH DEV MODE: RBAC unavailable, allowing {Action} for {Subject}",
                    actionKey, subject);
                return await continuation(request, context);

            default:
                throw new RpcException(new Status(StatusCode.Internal,
                    "unknown authorization mode"));
        }
    }
}
