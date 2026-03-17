using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Options;

namespace Globular.Runtime;

public class GlobularServiceContext : IGlobularServiceContext
{
    private readonly GlobularHostOptions _options;

    public GlobularServiceContext(IOptions<GlobularHostOptions> options, IHostEnvironment env)
    {
        _options = options.Value;
        Environment = env.EnvironmentName;
        StartTime = DateTimeOffset.UtcNow;
    }

    public GlobularServiceIdentity ServiceIdentity => _options.Service;
    public GlobularRuntimeOptions RuntimeOptions => _options.Runtime;
    public GlobularNetworkOptions NetworkOptions => _options.Network;
    public string Environment { get; }
    public DateTimeOffset StartTime { get; }
    public string BindAddress => _options.Network.GrpcBindAddress;
    public int GrpcPort => _options.Network.GrpcPort;
}
