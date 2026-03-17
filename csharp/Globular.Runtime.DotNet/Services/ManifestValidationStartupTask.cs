using Microsoft.Extensions.Logging;

namespace Globular.Runtime;

/// <summary>
/// Built-in startup task that validates the resolved service manifest on boot.
/// </summary>
public class ManifestValidationStartupTask : IGlobularStartupTask
{
    private readonly IGlobularManifestProvider _manifestProvider;
    private readonly ILogger<ManifestValidationStartupTask> _logger;

    public ManifestValidationStartupTask(
        IGlobularManifestProvider manifestProvider,
        ILogger<ManifestValidationStartupTask> logger)
    {
        _manifestProvider = manifestProvider;
        _logger = logger;
    }

    public string Name => "ManifestValidation";
    public int Order => 0;

    public Task ExecuteAsync(CancellationToken cancellationToken = default)
    {
        var manifest = _manifestProvider.Manifest;
        var errors = GlobularManifestValidator.Validate(manifest);

        if (errors.Count > 0)
        {
            foreach (var error in errors)
            {
                _logger.LogError("Manifest validation: {Error}", error);
            }
            throw new InvalidOperationException(
                $"Service manifest is invalid: {string.Join("; ", errors)}");
        }

        _logger.LogInformation("Service manifest validated: {Name} v{Version}",
            manifest.Identity.Name, manifest.Identity.Version);

        return Task.CompletedTask;
    }
}
