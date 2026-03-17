namespace Globular.Runtime;

public class GlobularDiscoveryOptions
{
    public bool Enabled { get; set; }
    public bool RegisterOnStartup { get; set; }
    public string? RegistryAddress { get; set; }
}
