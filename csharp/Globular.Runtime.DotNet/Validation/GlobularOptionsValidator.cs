namespace Globular.Runtime;

public static class GlobularOptionsValidator
{
    private static readonly HashSet<string> AllowedHealthModes = new(StringComparer.OrdinalIgnoreCase)
    {
        "grpc", "http", "none"
    };

    private static readonly HashSet<string> AllowedRestartPolicies = new(StringComparer.OrdinalIgnoreCase)
    {
        "always", "on-failure", "never"
    };

    public static List<string> Validate(GlobularHostOptions options)
    {
        var errors = new List<string>();

        // Service identity
        if (string.IsNullOrWhiteSpace(options.Service.Name))
            errors.Add("Service.Name is required");

        if (string.IsNullOrWhiteSpace(options.Service.Version))
            errors.Add("Service.Version is required");

        if (string.IsNullOrWhiteSpace(options.Service.PublisherId))
            errors.Add("Service.PublisherId is required");

        // Network
        if (options.Network.GrpcPort <= 0 || options.Network.GrpcPort > 65535)
            errors.Add($"Network.GrpcPort must be between 1 and 65535, got {options.Network.GrpcPort}");

        if (string.IsNullOrWhiteSpace(options.Network.GrpcBindAddress))
            errors.Add("Network.GrpcBindAddress is required");

        // Runtime
        if (string.IsNullOrWhiteSpace(options.Runtime.RuntimeKind))
            errors.Add("Runtime.RuntimeKind is required");

        // Health
        var healthMode = options.Health.Mode?.ToLowerInvariant() ?? "";
        if (!string.IsNullOrEmpty(healthMode) && !AllowedHealthModes.Contains(healthMode))
            errors.Add($"Health.Mode must be one of: {string.Join(", ", AllowedHealthModes)}, got '{options.Health.Mode}'");

        // Lifecycle
        var restartPolicy = options.Lifecycle.RestartPolicy?.ToLowerInvariant() ?? "";
        if (!string.IsNullOrEmpty(restartPolicy) && !AllowedRestartPolicies.Contains(restartPolicy))
            errors.Add($"Lifecycle.RestartPolicy must be one of: {string.Join(", ", AllowedRestartPolicies)}, got '{options.Lifecycle.RestartPolicy}'");

        return errors;
    }
}
