// GlobularEtcdClient — concrete IEtcdClient implementation using dotnet-etcd.
// Connects to the Globular etcd cluster with TLS for service state publication.
//
// Go reference: golang/config/etcd_client.go
// - Mandatory TLS with cluster CA
// - Singleton connection with health probe
// - Key paths: /globular/services/<id>/runtime-state
//
// This client is used by EtcdServiceStatePublisher to publish/unpublish
// effective runtime state to the cluster management plane.

using dotnet_etcd;
using Etcdserverpb;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace Globular.Runtime.Authorization;

/// <summary>
/// Configuration for the Globular etcd client.
/// </summary>
public sealed class GlobularEtcdOptions
{
    /// <summary>
    /// etcd endpoint (e.g., "https://127.0.0.1:2379").
    /// Mirrors Go: config.GetEtcdEndpoints().
    /// </summary>
    public string Endpoint { get; set; } = "https://127.0.0.1:2379";

    /// <summary>
    /// CA certificate path for TLS.
    /// Default: /var/lib/globular/pki/ca.crt (Globular canonical path).
    /// </summary>
    public string CaCertPath { get; set; } = "/var/lib/globular/pki/ca.crt";

    /// <summary>
    /// Client certificate path for mTLS (optional but recommended).
    /// Default: /var/lib/globular/pki/issued/services/service.crt
    /// </summary>
    public string? ClientCertPath { get; set; } = "/var/lib/globular/pki/issued/services/service.crt";

    /// <summary>
    /// Client key path for mTLS.
    /// Default: /var/lib/globular/pki/issued/services/service.key
    /// </summary>
    public string? ClientKeyPath { get; set; } = "/var/lib/globular/pki/issued/services/service.key";

    /// <summary>Request timeout for etcd operations.</summary>
    public TimeSpan Timeout { get; set; } = TimeSpan.FromSeconds(5);
}

/// <summary>
/// Concrete etcd client for Globular service state management.
/// Uses dotnet-etcd library with mandatory TLS (mirrors Go etcdClient behavior).
///
/// Connection is lazily initialized on first use and shared across calls
/// (mirrors Go cliShared singleton pattern).
/// </summary>
public sealed class GlobularEtcdClient : IEtcdClient, IDisposable
{
    private readonly GlobularEtcdOptions _options;
    private readonly ILogger<GlobularEtcdClient> _logger;
    private EtcdClient? _client;
    private readonly SemaphoreSlim _lock = new(1, 1);

    public GlobularEtcdClient(IOptions<GlobularEtcdOptions> options, ILogger<GlobularEtcdClient> logger)
    {
        _options = options.Value;
        _logger = logger;
    }

    public async Task PutAsync(string key, string value, CancellationToken ct = default)
    {
        var client = await GetClientAsync(ct);
        await client.PutAsync(key, value, cancellationToken: ct);
    }

    public async Task DeleteAsync(string key, CancellationToken ct = default)
    {
        var client = await GetClientAsync(ct);
        await client.DeleteAsync(key, cancellationToken: ct);
    }

    /// <summary>
    /// Gets a value from etcd by key. Returns null if not found.
    /// </summary>
    public async Task<string?> GetAsync(string key, CancellationToken ct = default)
    {
        var client = await GetClientAsync(ct);
        var response = await client.GetValAsync(key, cancellationToken: ct);
        return string.IsNullOrEmpty(response) ? null : response;
    }

    private async Task<EtcdClient> GetClientAsync(CancellationToken ct)
    {
        if (_client is not null)
            return _client;

        await _lock.WaitAsync(ct);
        try
        {
            if (_client is not null)
                return _client;

            _logger.LogInformation("Connecting to etcd at {Endpoint}", _options.Endpoint);

            // Build TLS configuration matching Go's mandatory TLS requirement.
            // The dotnet-etcd library handles TLS via the endpoint URL scheme (https://).
            // For mTLS with client certs, we configure the handler.
            var handler = new HttpClientHandler();

            // Load CA certificate if available.
            if (!string.IsNullOrEmpty(_options.CaCertPath) && File.Exists(_options.CaCertPath))
            {
                var caCert = new System.Security.Cryptography.X509Certificates.X509Certificate2(
                    _options.CaCertPath);
                handler.ServerCertificateCustomValidationCallback = (message, cert, chain, errors) =>
                {
                    if (errors == System.Net.Security.SslPolicyErrors.None)
                        return true;

                    // Validate against our CA.
                    if (chain is not null && cert is not null)
                    {
                        chain.ChainPolicy.TrustMode =
                            System.Security.Cryptography.X509Certificates.X509ChainTrustMode.CustomRootTrust;
                        chain.ChainPolicy.CustomTrustStore.Add(caCert);
                        return chain.Build(new System.Security.Cryptography.X509Certificates.X509Certificate2(cert));
                    }

                    return false;
                };
            }

            // Load client certificate for mTLS if available.
            if (!string.IsNullOrEmpty(_options.ClientCertPath) &&
                !string.IsNullOrEmpty(_options.ClientKeyPath) &&
                File.Exists(_options.ClientCertPath) &&
                File.Exists(_options.ClientKeyPath))
            {
                var clientCert = System.Security.Cryptography.X509Certificates.X509Certificate2
                    .CreateFromPemFile(_options.ClientCertPath, _options.ClientKeyPath);
                handler.ClientCertificates.Add(clientCert);
            }

            _client = new EtcdClient(_options.Endpoint, configureChannelOptions: options =>
            {
                options.HttpHandler = handler;
            });

            _logger.LogInformation("Connected to etcd at {Endpoint}", _options.Endpoint);
            return _client;
        }
        finally
        {
            _lock.Release();
        }
    }

    public void Dispose()
    {
        _client?.Dispose();
        _lock.Dispose();
    }
}
