import "@typespec/http";

using TypeSpec.Http;

namespace Platrium;

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

@error
model InvalidJsonError {
    @statusCode _: 400;
    message: string;
}

@error
model InternalServerError {
    @statusCode _: 500;
    message: string;
}

@route("/fs")
@tag("FileSystem")
interface FileSystem {
    @route("createfile")
    @post
    @summary("Create a new file")
    @doc("Creates a New File for the Current User")
    createFile(@body body: CreateFileRequest):
        | CreateFileResponse
        | InvalidJsonError
        | InternalServerError;
}
