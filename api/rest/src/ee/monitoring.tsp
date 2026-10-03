import "@typespec/http";
import "@typespec/openapi";

using TypeSpec.Http;
using TypeSpec.OpenAPI;

namespace Platrium.EE.Monitoring;

model HealthResponse {
    @statusCode _: 200;
    status: string;
}

@route("/ee/monitoring")
@tag("ee")
interface Monitoring {
    @route("/health")
    @get
    @summary("Enterprise health check")
    @doc("Verifies that the Enterprise API is loaded and functioning.")
    checkHealth(): HealthResponse;
}
