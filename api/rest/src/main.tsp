import "@typespec/http";
import "./fsops_file.tsp";

using TypeSpec.Http;

@service(#{ title: "Platrium Core API" })
@server("http://localhost:3000/api", "Local development server")
namespace Platrium;
