// GlobularAuthorizationInterceptor — mirrors Go interceptors/ServerInterceptors.go.
// Centralized gRPC server interceptor for authorization enforcement.
//
// Runtime flow:
//   1. Extract gRPC method from ServerCallContext
//   2. Resolve method → stable action key via ActionResolver
//   3. Extract auth token from metadata
//   4. Check action permission via RBAC (IRbacClient)
//   5. Expand resource template from request fields
//   6. Check resource-path permission via RBAC
//   7. Allow or deny

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

    /// <summary>Check if the subject has a role that grants the given action.</summary>
    Task<bool> CheckRoleBindingAsync(string subject, string action, CancellationToken ct = default);
}

/// <summary>Resource path + permission for RBAC resource-level checks.</summary>
public sealed record ResourcePathCheck(string Path, string Permission);

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
    private readonly ILogger<GlobularAuthorizationInterceptor> _logger;

    /// <summary>
    /// Methods that do not require authentication (health checks, reflection, etc.).
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
        ILogger<GlobularAuthorizationInterceptor> logger)
    {
        _resolver = resolver;
        _rbac = rbac;
        _logger = logger;
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

        // Extract subject from metadata.
        var subject = ExtractSubject(context);
        if (string.IsNullOrEmpty(subject))
        {
            _logger.LogDebug("Auth: no subject for {Method}, allowing (pre-auth)", method);
            return await continuation(request, context);
        }

        // Check role binding.
        var roleAllowed = await _rbac.CheckRoleBindingAsync(subject, actionKey);
        if (roleAllowed)
        {
            _logger.LogDebug("Auth: role binding granted for {Subject} on {Action}", subject, actionKey);
            return await continuation(request, context);
        }

        // Check action permission via RBAC.
        var resourceChecks = ExpandResourceTemplate(method, request);
        var (allowed, denied) = await _rbac.ValidateActionAsync(
            actionKey, subject, "ACCOUNT", resourceChecks);

        if (allowed && !denied)
            return await continuation(request, context);

        _logger.LogWarning("Auth: denied {Subject} for {Action} ({Method})",
            subject, actionKey, method);
        throw new RpcException(new Status(StatusCode.PermissionDenied,
            $"permission denied: {actionKey}"));
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
        var subject = ExtractSubject(context);

        if (!string.IsNullOrEmpty(subject))
        {
            var roleAllowed = await _rbac.CheckRoleBindingAsync(subject, actionKey);
            if (!roleAllowed)
            {
                var resourceChecks = ExpandResourceTemplate(method, request);
                var (allowed, denied) = await _rbac.ValidateActionAsync(
                    actionKey, subject, "ACCOUNT", resourceChecks);
                if (!allowed || denied)
                {
                    throw new RpcException(new Status(StatusCode.PermissionDenied,
                        $"permission denied: {actionKey}"));
                }
            }
        }

        await continuation(request, responseStream, context);
    }

    // ── Helpers ─────────────────────────────────────────────────────────────

    private static string? ExtractSubject(ServerCallContext context)
    {
        var entry = context.RequestHeaders.Get("token");
        if (entry is not null)
            return entry.Value; // simplified — real impl would decode JWT

        entry = context.RequestHeaders.Get("authorization");
        if (entry is not null && entry.Value.StartsWith("Bearer ", StringComparison.OrdinalIgnoreCase))
            return entry.Value[7..]; // simplified

        return null;
    }

    /// <summary>
    /// Expands the resource template for the given method using request field values.
    /// Returns null if no template exists or expansion is not possible.
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
            return null; // deny will happen at RBAC level
        }
    }

    /// <summary>
    /// Extracts string properties from a protobuf message using reflection.
    /// Uses the property name as-is (C# protobuf codegen uses PascalCase,
    /// but we need camelCase for template {placeholders}).
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

            // Store both PascalCase and camelCase for template matching.
            fields[prop.Name] = value;
            var camelCase = char.ToLowerInvariant(prop.Name[0]) + prop.Name[1..];
            fields[camelCase] = value;
        }

        return fields;
    }
}
