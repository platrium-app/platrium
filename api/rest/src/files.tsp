import "@typespec/http";
import "./errors.tsp";

using TypeSpec.Http;

namespace Platrium.Files;

model CreateFileRequest {
    @doc("ID of the parent folder or drive")
    parent_id: string;

    @doc("Name of the file")
    file_name: string;

    @doc("Array of chunk hashes")
    hashes: string[];
}

model CreateFileResponse {
    @statusCode _: 201;
    file_id: string;
}

@route("/files")
@tag("Files")
interface Files {
    @route("create")
    @post
    @summary("Create a new file")
    @doc("Creates a New File for the Current User")
    createFile(@body body: CreateFileRequest):
        | CreateFileResponse
        | Platrium.Errors.MissingChunks
        | Platrium.Errors.EngineInternal;
}
