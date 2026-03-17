namespace Globular.Runtime;

public class GlobularHealthReporter : IGlobularHealthReporter
{
    private volatile GlobularHealthState _state = GlobularHealthState.Starting;
    private volatile string? _reason;

    public GlobularHealthState CurrentState => _state;
    public string? UnhealthyReason => _reason;

    public void SetHealthy()
    {
        _reason = null;
        _state = GlobularHealthState.Healthy;
    }

    public void SetUnhealthy(string reason)
    {
        _reason = reason;
        _state = GlobularHealthState.Unhealthy;
    }
}
