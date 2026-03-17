namespace Globular.Runtime;

/// <summary>
/// Provides runtime context for the current Globular-managed service.
/// Injectable into handlers and services.
/// </summary>
public interface IGlobularServiceContext
{
    GlobularServiceIdentity ServiceIdentity { get; }
    GlobularRuntimeOptions RuntimeOptions { get; }
    GlobularNetworkOptions NetworkOptions { get; }
    string Environment { get; }
    DateTimeOffset StartTime { get; }
    string BindAddress { get; }
    int GrpcPort { get; }
}
