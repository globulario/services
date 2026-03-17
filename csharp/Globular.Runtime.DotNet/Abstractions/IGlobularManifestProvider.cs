namespace Globular.Runtime;

/// <summary>
/// Provides the resolved service manifest at runtime.
/// </summary>
public interface IGlobularManifestProvider
{
    GlobularServiceManifest Manifest { get; }
}
