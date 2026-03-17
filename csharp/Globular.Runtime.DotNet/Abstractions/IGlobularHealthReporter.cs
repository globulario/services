namespace Globular.Runtime;

/// <summary>
/// Reports and manages service health state.
/// </summary>
public interface IGlobularHealthReporter
{
    GlobularHealthState CurrentState { get; }
    string? UnhealthyReason { get; }
    void SetHealthy();
    void SetUnhealthy(string reason);
}
