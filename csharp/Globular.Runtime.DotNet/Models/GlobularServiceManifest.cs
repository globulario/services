namespace Globular.Runtime;

/// <summary>
/// Complete service manifest describing a Globular-managed service.
/// Can be loaded from a manifest JSON file or constructed from host options.
/// </summary>
public class GlobularServiceManifest
{
    public GlobularServiceIdentity Identity { get; set; } = new();
    public GlobularRuntimeOptions Runtime { get; set; } = new();
    public GlobularNetworkOptions Network { get; set; } = new();
    public GlobularHealthOptions Health { get; set; } = new();
    public GlobularDiscoveryOptions Discovery { get; set; } = new();
    public GlobularLifecycleOptions Lifecycle { get; set; } = new();

    /// <summary>
    /// Creates a manifest from host options.
    /// </summary>
    public static GlobularServiceManifest FromOptions(GlobularHostOptions options)
    {
        return new GlobularServiceManifest
        {
            Identity = options.Service,
            Runtime = options.Runtime,
            Network = options.Network,
            Health = options.Health,
            Discovery = options.Discovery,
            Lifecycle = options.Lifecycle,
        };
    }
}
