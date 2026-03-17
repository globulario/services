namespace Globular.Runtime;

public class GlobularRuntimeOptions
{
    public string RuntimeKind { get; set; } = "dotnet";
    public string Entrypoint { get; set; } = "";
    public string WorkingDirectory { get; set; } = ".";
}
