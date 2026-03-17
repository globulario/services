namespace Globular.Runtime;

public static class GlobularManifestValidator
{
    public static List<string> Validate(GlobularServiceManifest manifest)
    {
        var errors = new List<string>();

        if (string.IsNullOrWhiteSpace(manifest.Identity.Name))
            errors.Add("Identity.Name is required");

        if (string.IsNullOrWhiteSpace(manifest.Identity.Version))
            errors.Add("Identity.Version is required");

        if (string.IsNullOrWhiteSpace(manifest.Identity.PublisherId))
            errors.Add("Identity.PublisherId is required");

        if (manifest.Network.GrpcPort <= 0 || manifest.Network.GrpcPort > 65535)
            errors.Add($"Network.GrpcPort must be between 1 and 65535, got {manifest.Network.GrpcPort}");

        if (string.IsNullOrWhiteSpace(manifest.Network.GrpcBindAddress))
            errors.Add("Network.GrpcBindAddress is required");

        if (string.IsNullOrWhiteSpace(manifest.Runtime.RuntimeKind))
            errors.Add("Runtime.RuntimeKind is required");

        return errors;
    }
}
