namespace Globular.Runtime;

public class GlobularServiceIdentity
{
    public string ServiceId { get; set; } = "";
    public string Name { get; set; } = "";
    public string DisplayName { get; set; } = "";
    public string Version { get; set; } = "0.0.0";
    public string PublisherId { get; set; } = "";
    public string Description { get; set; } = "";
    public string Domain { get; set; } = "";
    public string ProtoPackage { get; set; } = "";
    public string ProtoService { get; set; } = "";
}
