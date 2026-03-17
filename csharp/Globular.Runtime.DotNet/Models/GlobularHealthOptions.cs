namespace Globular.Runtime;

public class GlobularHealthOptions
{
    public bool Enabled { get; set; } = true;
    public string Mode { get; set; } = "grpc";
    public string? Endpoint { get; set; }
}
