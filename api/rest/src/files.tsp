import "@typespec/http";
import "./errors.tsp";

using TypeSpec.Http;

namespace Platrium.Files;

/* Custom Scalar Validation Types */

@doc("Strict SHA-256 Hex Hash validation pattern (64 characters, lowercase hex)")
@pattern("^[a-f0-9]{64}$")
scalar ChunkHash extends string;

/* Upload Session Core Models */

model UploadSessionInitRequest {
    @doc("ID of the parent folder or drive")
    parent_id: string;

    @doc("Name of the file")
    file_name: string;

    @doc("Size of the file in bytes. Max size 2TiB.")
    @maxValue(2199023255552)
    @minValue(1)
    file_size: int64;
}

model UploadSessionInitResponse {
    @statusCode _: 201;

    @doc("Cryptographically sealed JWT token acting as the session_id passport")
    session_id: string;
}

model UploadSessionChunksRequest {
    @doc("Batch of SHA-256 chunk hashes to authorize. Capped at 512 items (2GiB window).")
    @minItems(1)
    @maxItems(512)
    hashes: ChunkHash[];

    @doc("Flag indicating if last element in hashes is the last (EOF) chunk in the file. Allows Chunk to be < 4MiB.")
    contains_eof_chunk?: boolean;
}

model UploadSessionPresignedChunk {
    @doc("The client-direct Presigned upload URL. Null if the chunk already exists.")
    upload_url: string | null;

    @doc("Symmetric HMAC authorization receipt bound to this specific session and chunk.")
    receipt: string;
}

model UploadSessionChunksResponse {
    @statusCode _: 200;

    @doc("A key-value map mapping each authorized ChunkHash string directly to its metadata and receipts.")
    chunks: Record<UploadSessionPresignedChunk>;
}

model UploadSessionCommitChunk {
    @doc("SHA-256 Hash of Chunk Previously used to obtain Presign Status")
    hash: ChunkHash;

    @doc("Cryptographic HMAC receipt authorized by the server for this chunk")
    receipt: string;
}

model UploadSessionCommitRequest {
    @doc("Strictly ordered list of chunks forming the master file manifest sequence. Max items support a 2TiB file.")
    @minItems(1)
    @maxItems(524288)
    chunks: UploadSessionCommitChunk[];
}

model UploadSessionCommitResponse {
    @statusCode _: 200;

    @doc("The created FileNode ID")
    file_id: string;
}

/* Upload Session Routes */

@route("/files/uploadsession")
@tag("Files")
interface UploadSession {
    @post
    @summary("Initialize a new upload session")
    @doc("Initialize a file upload session. Verifies destination folder ACLs and returns a sealed passport token.")
    initialize(@body body: UploadSessionInitRequest):
        | UploadSessionInitResponse
        | Platrium.Errors.EngineInternal;

    @route("/chunks")
    @post
    @summary("Presign a batch of chunks for upload")
    @doc("Batch request presigned URLs and cryptographic authorization receipts for chunk uploading.")
    chunks(
        @header("x-platrium-uploadsession") sessionId: string,
        @body body: UploadSessionChunksRequest,
    ): UploadSessionChunksResponse | Platrium.Errors.EngineInternal;

    @route("/commit")
    @post
    @summary("Commit an upload session and finalize the file")
    @doc("Commit an upload session and finalize the file layout using the provided cryptographic receipts.")
    commit(
        @header("x-platrium-uploadsession") sessionId: string,
        @body body: UploadSessionCommitRequest,
    ): UploadSessionCommitResponse | Platrium.Errors.EngineInternal;
}
