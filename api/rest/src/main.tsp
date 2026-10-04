import "@typespec/http";
import "./files.tsp";
import "./auth.tsp";

using TypeSpec.Http;

@service(#{ title: "Platrium Core API" })
@server("http://localhost:3000/api", "Local development server")
namespace Platrium;
