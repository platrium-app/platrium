import "@typespec/http";

using TypeSpec.Http;

namespace Platrium.Errors;

@error
model EngineInternal {
    @statusCode _: 500;
    debuginfo: string;
}

@error
model MissingChunks {
    @statusCode _: 404;
    missing_chunks: string[];
}
