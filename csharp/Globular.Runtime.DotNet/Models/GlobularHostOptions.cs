namespace Globular.Runtime;

/// <summary>
/// Root options model bound from the "Globular" configuration section.
/// </summary>
public class GlobularHostOptions
{
    public GlobularServiceIdentity Service { get; set; } = new();
    public GlobularNetworkOptions Network { get; set; } = new();
    public GlobularHealthOptions Health { get; set; } = new();
    public GlobularDiscoveryOptions Discovery { get; set; } = new();
    public GlobularLifecycleOptions Lifecycle { get; set; } = new();
    public GlobularRuntimeOptions Runtime { get; set; } = new();
    public GlobularConfigOptions Config { get; set; } = new();
}
