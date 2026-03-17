using System.Text.Json;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime;

/// <summary>
/// Resolves the service manifest from a JSON file if present, or from host options as fallback.
/// </summary>
public class GlobularManifestProvider : IGlobularManifestProvider
{
    private readonly Lazy<GlobularServiceManifest> _manifest;

    public GlobularManifestProvider(
        IOptions<GlobularHostOptions> options,
        ILogger<GlobularManifestProvider> logger)
    {
        _manifest = new Lazy<GlobularServiceManifest>(() =>
        {
            var opts = options.Value;
            var configFile = opts.Config.ConfigFileName;

            // Try to load a manifest JSON file from well-known locations
            var manifestPaths = new[]
            {
                Path.Combine(Directory.GetCurrentDirectory(), $"{opts.Service.ProtoPackage}.service.manifest.json"),
                Path.Combine(Directory.GetCurrentDirectory(), "service.manifest.json"),
            };

            foreach (var path in manifestPaths)
            {
                if (File.Exists(path))
                {
                    try
                    {
                        var json = File.ReadAllText(path);
                        var manifest = JsonSerializer.Deserialize<GlobularServiceManifest>(json, new JsonSerializerOptions
                        {
                            PropertyNameCaseInsensitive = true
                        });
                        if (manifest != null)
                        {
                            logger.LogInformation("Loaded service manifest from {Path}", path);
                            return manifest;
                        }
                    }
                    catch (Exception ex)
                    {
                        logger.LogWarning(ex, "Failed to load manifest from {Path}, falling back to options", path);
                    }
                }
            }

            // Fallback: build manifest from host options
            logger.LogDebug("No manifest file found, building manifest from configuration");
            return GlobularServiceManifest.FromOptions(opts);
        });
    }

    public GlobularServiceManifest Manifest => _manifest.Value;
}
