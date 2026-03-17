namespace Globular.Runtime;

public class GlobularNetworkOptions
{
    public string GrpcBindAddress { get; set; } = "0.0.0.0";
    public int GrpcPort { get; set; } = 50051;
    public string? PublicAddress { get; set; }
    public bool TlsEnabled { get; set; }
}
