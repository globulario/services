namespace Globular.Runtime;

/// <summary>
/// Optional marker interface for Globular-managed gRPC service implementations.
/// </summary>
public interface IGlobularManagedService
{
    string ServiceName { get; }
    string ServiceVersion { get; }
}
