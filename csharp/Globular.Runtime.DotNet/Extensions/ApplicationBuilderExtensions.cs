using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace Globular.Runtime;

public static class ApplicationBuilderExtensions
{
    /// <summary>
    /// Maps the Globular health endpoint based on configured health mode.
    /// - Mode "http": maps GET /health
    /// - Mode "none" or Health.Enabled == false: no endpoint mapped
    /// - Mode "grpc": maps HTTP /health in v1 (gRPC health is future work)
    /// </summary>
    public static WebApplication MapGlobularHealth(this WebApplication app)
    {
        var options = app.Services.GetRequiredService<IOptions<GlobularHostOptions>>().Value;

        if (!options.Health.Enabled)
            return app;

        var mode = options.Health.Mode?.ToLowerInvariant() ?? "http";
        if (mode == "none")
            return app;

        // Both "http" and "grpc" map an HTTP health endpoint in v1.
        // Full gRPC health (grpc.health.v1) is future work.
        var path = options.Health.Endpoint ?? "/health";

        app.MapGet(path, (IGlobularHealthReporter health) =>
        {
            var state = health.CurrentState;
            var body = new
            {
                status = state.ToString().ToLowerInvariant(),
                reason = health.UnhealthyReason
            };

            return state == GlobularHealthState.Healthy
                ? Results.Ok(body)
                : Results.Json(body, statusCode: 503);
        });

        return app;
    }
}
