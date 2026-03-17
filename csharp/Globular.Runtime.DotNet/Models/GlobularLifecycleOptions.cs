namespace Globular.Runtime;

public class GlobularLifecycleOptions
{
    public bool KeepAlive { get; set; } = true;
    public string RestartPolicy { get; set; } = "on-failure";
}
