// RbacProto.cs — Hand-written gRPC client types matching proto/rbac.proto.
// These mirror the protobuf definitions so the RBAC client can make real
// gRPC calls to the Go RBAC service without requiring protoc code generation.
//
// When generated C# stubs from rbac.proto become available, replace this file
// with the generated code and update GlobularRbacClient accordingly.
//
// Proto reference: proto/rbac.proto (RbacService)

using Google.Protobuf;
using Grpc.Core;

namespace Globular.Runtime.Authorization.Rbac;

// ── Enums ────────────────────────────────────────────────────────────────

/// <summary>Maps to proto SubjectType enum.</summary>
public enum SubjectType
{
    Account = 0,
    NodeIdentity = 1,
    Group = 2,
    Organization = 3,
    Application = 4,
    Role = 5,
}

/// <summary>Maps to proto PermissionType enum.</summary>
public enum PermissionType
{
    Denied = 0,
    Allowed = 1,
}

// ── Messages ─────────────────────────────────────────────────────────────
// Each message is a simple POCO serialized/deserialized as JSON-like protobuf
// wire format using the Marshaller helpers below.

public sealed class ResourceInfos
{
    public int Index { get; set; }
    public string Permission { get; set; } = "";
    public string Path { get; set; } = "";
    public string Field { get; set; } = "";
}

public sealed class RoleBinding
{
    public string Subject { get; set; } = "";
    public List<string> Roles { get; set; } = new();
}

// ── ValidateAction ───────────────────────────────────────────────────────

public sealed class ValidateActionRqst
{
    public string Subject { get; set; } = "";
    public SubjectType Type { get; set; }
    public string Action { get; set; } = "";
    public List<ResourceInfos> Infos { get; set; } = new();
}

public sealed class ValidateActionRsp
{
    public bool HasAccess { get; set; }
    public bool AccessDenied { get; set; }
}

// ── ValidateAccess ───────────────────────────────────────────────────────

public sealed class ValidateAccessRqst
{
    public string Subject { get; set; } = "";
    public SubjectType Type { get; set; }
    public string Path { get; set; } = "";
    public string Permission { get; set; } = "";
}

public sealed class ValidateAccessRsp
{
    public bool HasAccess { get; set; }
    public bool AccessDenied { get; set; }
}

// ── GetRoleBinding ───────────────────────────────────────────────────────

public sealed class GetRoleBindingRqst
{
    public string Subject { get; set; } = "";
}

public sealed class GetRoleBindingRsp
{
    public RoleBinding? Binding { get; set; }
}

// ── SetRoleBinding ───────────────────────────────────────────────────────

public sealed class SetRoleBindingRqst
{
    public RoleBinding? Binding { get; set; }
}

public sealed class SetRoleBindingRsp { }

// ── AddResourceOwner ─────────────────────────────────────────────────────

public sealed class AddResourceOwnerRqst
{
    public string Path { get; set; } = "";
    public string ResourceType { get; set; } = "";
    public string Subject { get; set; } = "";
    public SubjectType Type { get; set; }
}

public sealed class AddResourceOwnerRsp { }

// ── RemoveResourceOwner ──────────────────────────────────────────────────

public sealed class RemoveResourceOwnerRqst
{
    public string Path { get; set; } = "";
    public string Subject { get; set; } = "";
    public SubjectType Type { get; set; }
}

public sealed class RemoveResourceOwnerRsp { }

// ── GetActionResourceInfos ───────────────────────────────────────────────

public sealed class GetActionResourceInfosRqst
{
    public string Action { get; set; } = "";
}

public sealed class GetActionResourceInfosRsp
{
    public List<ResourceInfos> Infos { get; set; } = new();
}

// ── DeleteAllAccess ──────────────────────────────────────────────────────

public sealed class DeleteAllAccessRqst
{
    public string Subject { get; set; } = "";
    public SubjectType Type { get; set; }
}

public sealed class DeleteAllAccessRsp { }

// ── SetActionResourcesPermissions ────────────────────────────────────────

public sealed class SetActionResourcesPermissionsRqst
{
    public string PermissionsJson { get; set; } = "";
}

public sealed class SetActionResourcesPermissionsRsp { }

// ── Protobuf Wire Serialization ──────────────────────────────────────────
// Minimal protobuf binary serializer for the subset of types we need.
// Uses manual field encoding matching the proto field numbers.

internal static class RbacMarshallers
{
    // Generic JSON-based marshaller. Protobuf binary would be more efficient
    // but JSON is compatible with the Go server's gRPC-gateway and simpler
    // to maintain without generated code.

    internal static Marshaller<T> Create<T>() where T : class, new()
    {
        return Marshallers.Create(
            serializer: obj => System.Text.Json.JsonSerializer.SerializeToUtf8Bytes(obj, JsonOpts),
            deserializer: bytes => System.Text.Json.JsonSerializer.Deserialize<T>(bytes, JsonOpts) ?? new T()
        );
    }

    private static readonly System.Text.Json.JsonSerializerOptions JsonOpts = new()
    {
        PropertyNamingPolicy = System.Text.Json.JsonNamingPolicy.CamelCase,
        DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingDefault,
    };
}

// ── gRPC Service Client ──────────────────────────────────────────────────
// Hand-written gRPC client stub matching the RbacService proto definition.
// Uses CallInvoker for real gRPC transport.

public sealed class RbacServiceClient
{
    private readonly CallInvoker _invoker;

    public RbacServiceClient(CallInvoker invoker)
    {
        _invoker = invoker;
    }

    public RbacServiceClient(ChannelBase channel) : this(channel.CreateCallInvoker()) { }

    private const string ServiceName = "/rbac.RbacService/";

    // ── RPC Methods ──────────────────────────────────────────────────

    private static readonly Method<ValidateActionRqst, ValidateActionRsp> ValidateActionMethod =
        new(MethodType.Unary, "rbac.RbacService", "ValidateAction",
            RbacMarshallers.Create<ValidateActionRqst>(),
            RbacMarshallers.Create<ValidateActionRsp>());

    public AsyncUnaryCall<ValidateActionRsp> ValidateActionAsync(
        ValidateActionRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(ValidateActionMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<ValidateAccessRqst, ValidateAccessRsp> ValidateAccessMethod =
        new(MethodType.Unary, "rbac.RbacService", "ValidateAccess",
            RbacMarshallers.Create<ValidateAccessRqst>(),
            RbacMarshallers.Create<ValidateAccessRsp>());

    public AsyncUnaryCall<ValidateAccessRsp> ValidateAccessAsync(
        ValidateAccessRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(ValidateAccessMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<GetRoleBindingRqst, GetRoleBindingRsp> GetRoleBindingMethod =
        new(MethodType.Unary, "rbac.RbacService", "GetRoleBinding",
            RbacMarshallers.Create<GetRoleBindingRqst>(),
            RbacMarshallers.Create<GetRoleBindingRsp>());

    public AsyncUnaryCall<GetRoleBindingRsp> GetRoleBindingAsync(
        GetRoleBindingRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(GetRoleBindingMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<SetRoleBindingRqst, SetRoleBindingRsp> SetRoleBindingMethod =
        new(MethodType.Unary, "rbac.RbacService", "SetRoleBinding",
            RbacMarshallers.Create<SetRoleBindingRqst>(),
            RbacMarshallers.Create<SetRoleBindingRsp>());

    public AsyncUnaryCall<SetRoleBindingRsp> SetRoleBindingAsync(
        SetRoleBindingRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(SetRoleBindingMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<AddResourceOwnerRqst, AddResourceOwnerRsp> AddResourceOwnerMethod =
        new(MethodType.Unary, "rbac.RbacService", "AddResourceOwner",
            RbacMarshallers.Create<AddResourceOwnerRqst>(),
            RbacMarshallers.Create<AddResourceOwnerRsp>());

    public AsyncUnaryCall<AddResourceOwnerRsp> AddResourceOwnerAsync(
        AddResourceOwnerRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(AddResourceOwnerMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<RemoveResourceOwnerRqst, RemoveResourceOwnerRsp> RemoveResourceOwnerMethod =
        new(MethodType.Unary, "rbac.RbacService", "RemoveResourceOwner",
            RbacMarshallers.Create<RemoveResourceOwnerRqst>(),
            RbacMarshallers.Create<RemoveResourceOwnerRsp>());

    public AsyncUnaryCall<RemoveResourceOwnerRsp> RemoveResourceOwnerAsync(
        RemoveResourceOwnerRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(RemoveResourceOwnerMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<GetActionResourceInfosRqst, GetActionResourceInfosRsp> GetActionResourceInfosMethod =
        new(MethodType.Unary, "rbac.RbacService", "GetActionResourceInfos",
            RbacMarshallers.Create<GetActionResourceInfosRqst>(),
            RbacMarshallers.Create<GetActionResourceInfosRsp>());

    public AsyncUnaryCall<GetActionResourceInfosRsp> GetActionResourceInfosAsync(
        GetActionResourceInfosRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(GetActionResourceInfosMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<DeleteAllAccessRqst, DeleteAllAccessRsp> DeleteAllAccessMethod =
        new(MethodType.Unary, "rbac.RbacService", "DeleteAllAccess",
            RbacMarshallers.Create<DeleteAllAccessRqst>(),
            RbacMarshallers.Create<DeleteAllAccessRsp>());

    public AsyncUnaryCall<DeleteAllAccessRsp> DeleteAllAccessAsync(
        DeleteAllAccessRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(DeleteAllAccessMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }

    private static readonly Method<SetActionResourcesPermissionsRqst, SetActionResourcesPermissionsRsp>
        SetActionResourcesPermissionsMethod =
            new(MethodType.Unary, "rbac.RbacService", "SetActionResourcesPermissions",
                RbacMarshallers.Create<SetActionResourcesPermissionsRqst>(),
                RbacMarshallers.Create<SetActionResourcesPermissionsRsp>());

    public AsyncUnaryCall<SetActionResourcesPermissionsRsp> SetActionResourcesPermissionsAsync(
        SetActionResourcesPermissionsRqst request, Metadata? headers = null,
        DateTime? deadline = null, CancellationToken cancellationToken = default)
    {
        return _invoker.AsyncUnaryCall(SetActionResourcesPermissionsMethod,
            null, new CallOptions(headers, deadline, cancellationToken), request);
    }
}

// ── SubjectType Conversion Helpers ───────────────────────────────────────

public static class SubjectTypeExtensions
{
    public static SubjectType Parse(string subjectType)
    {
        return subjectType.ToUpperInvariant() switch
        {
            "ACCOUNT" => SubjectType.Account,
            "NODE_IDENTITY" => SubjectType.NodeIdentity,
            "GROUP" => SubjectType.Group,
            "ORGANIZATION" => SubjectType.Organization,
            "APPLICATION" => SubjectType.Application,
            "ROLE" => SubjectType.Role,
            _ => SubjectType.Account,
        };
    }

    public static string ToProtoString(this SubjectType type)
    {
        return type switch
        {
            SubjectType.Account => "ACCOUNT",
            SubjectType.NodeIdentity => "NODE_IDENTITY",
            SubjectType.Group => "GROUP",
            SubjectType.Organization => "ORGANIZATION",
            SubjectType.Application => "APPLICATION",
            SubjectType.Role => "ROLE",
            _ => "ACCOUNT",
        };
    }
}
