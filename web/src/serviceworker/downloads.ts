/// <reference lib="webworker" />

import { PlatriumClient, DownloadDestination } from 'platrium-sdk';
import { createLogger } from '../lib/logging';

const logger = createLogger("Platrium SW");

declare const self: ServiceWorkerGlobalScope;

export function buildDownloadHeaders(
    session: { fileName: string; mimeType: string; fileSize: bigint },
    isForceDownload: boolean
): Headers {
    const headers = new Headers();
    headers.set('Server', 'platrium-serviceworker');
    headers.set('Content-Type', session.mimeType);
    headers.set('Accept-Ranges', 'bytes');
    headers.set('Content-Length', session.fileSize.toString());

    // Sanitize fallback filename to strictly ASCII-printable characters (remove non-ASCII, quotes, slashes)
    const safeFileName = session.fileName.replace(/[^\x20-\x7E]/g, '_').replace(/["/\\]/g, '_');

    // RFC 5987 standard for UTF-8 filenames in HTTP headers
    const encodedFileName = encodeURIComponent(session.fileName);

    if (isForceDownload) {
        headers.set('Content-Disposition', `attachment; filename="${safeFileName}"; filename*=UTF-8''${encodedFileName}`);
    } else {
        headers.set('Content-Disposition', `inline; filename="${safeFileName}"; filename*=UTF-8''${encodedFileName}`);
    }

    return headers;
}

let clientInstance: PlatriumClient | null = null;

function getClient(): PlatriumClient {
    if (!clientInstance) {
        const baseUrl = "http://localhost:3000/api";
        logger.debug(`Initializing singleton PlatriumClient with baseUrl: ${baseUrl}`);
        try {
            clientInstance = new PlatriumClient(baseUrl);
        } catch (e) {
            clientInstance = null;
            throw e;
        }
    }
    return clientInstance;
}

// TODO: This needs to be written better instead of parsing
export function createErrorResponse(err: unknown): Response {
    const errorMessage = err instanceof Error ? err.message : String(err);
    const isNotFound =
        errorMessage.toLowerCase().includes("404") ||
        errorMessage.toLowerCase().includes("not found");

    if (isNotFound) {
        return new Response(
            JSON.stringify({
                error: "FILE_NOT_FOUND",
                message: "The requested file was not found or is inaccessible.",
                details: errorMessage
            }),
            {
                status: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Server': 'platrium-serviceworker'
                }
            }
        );
    }

    const isBadGateway =
        errorMessage.includes("ApiError") ||
        errorMessage.includes("reqwest") ||
        errorMessage.includes("NetworkError") ||
        errorMessage.includes("Failed to fetch");

    const status = isBadGateway ? 502 : 500;
    const errorType = isBadGateway ? "BAD_GATEWAY" : "INTERNAL_SERVER_ERROR";

    return new Response(
        JSON.stringify({
            error: errorType,
            message: isBadGateway
                ? "Failed to communicate with storage backend or download stream."
                : "Internal SDK or Service Worker error occurred.",
            details: errorMessage,
            stack: err instanceof Error ? err.stack : undefined
        }),
        {
            status,
            headers: {
                'Content-Type': 'application/json',
                'Server': 'platrium-serviceworker'
            }
        }
    );
}

export async function handleDownloadRequest(event: FetchEvent, fileId: string, url: URL): Promise<Response> {
    const request = event.request;
    const method = request.method.toUpperCase();
    logger.info(`Intercepted ${method} download request for fileId: ${fileId}`);
    try {
        const client = getClient();

        logger.debug(`Requesting download session...`);
        const session = await client.files().createDownloadSession(fileId);

        logger.info(`Download session created successfully:`, {
            fileName: session.fileName,
            mimeType: session.mimeType,
            fileSize: Number(session.fileSize)
        });

        const isForceDownload = url.searchParams.get('dl') === '1';
        const headers = buildDownloadHeaders(session, isForceDownload);

        // Handle HTTP HEAD request: Return headers & metadata with empty body
        if (method === 'HEAD') {
            logger.info(`Responding to HEAD request with metadata headers`);
            return new Response(null, {
                status: 200,
                headers,
            });
        }

        const fileSize = session.fileSize; // bigint
        const rangeHeader = request.headers.get('Range');
        if (rangeHeader) {
            logger.debug(`Range header present: ${rangeHeader}`);
        }

        // Setup TransformStream using Web Streams API
        logger.debug(`Creating TransformStream...`);
        const { readable, writable } = new TransformStream();
        const destination = new DownloadDestination(writable);
        logger.debug(`Destination created successfully.`);

        if (rangeHeader) {
            // Handle HTTP Range request (e.g. from Video Player seeking)
            const match = rangeHeader.match(/bytes=(\d+)-(\d*)/);
            if (match) {
                const start = BigInt(match[1]);
                const endStr = match[2];
                let end = endStr ? BigInt(endStr) : fileSize - 1n;

                if (end >= fileSize) {
                    end = fileSize - 1n;
                }

                if (start > end || start >= fileSize) {
                    logger.warn(`Range Not Satisfiable: bytes ${start}-${end}/${fileSize}`);
                    return new Response(null, {
                        status: 416, // Range Not Satisfiable
                        headers: {
                            'Server': 'platrium-serviceworker',
                            'Content-Range': `bytes */${fileSize}`
                        }
                    });
                }

                const contentLength = end - start + 1n;
                headers.set('Content-Length', contentLength.toString());
                headers.set('Content-Range', `bytes ${start}-${end}/${fileSize}`);

                // Trigger background download via SDK and keep SW alive
                logger.info(`Triggering Range Download: bytes ${start}-${end}...`);
                const downloadPromise = session.streamRangeTo(destination, start, end)
                    .then(() => {
                        logger.info(`SDK streamRangeTo finished successfully. Closing writable stream.`);
                        writable.close();
                    })
                    .catch((err) => {
                        logger.error("SDK streamRangeTo failed:", err);
                        writable.abort(err).catch(e => logger.error(e));
                    });

                event.waitUntil(downloadPromise);

                logger.debug(`Returning 206 Partial Content Response`);
                return new Response(readable, {
                    status: 206, // Partial Content
                    headers
                });
            }
        }

        // Full file download
        headers.set('Content-Length', fileSize.toString());

        // Trigger background download via SDK and keep SW alive
        logger.info(`Triggering Full Download...`);
        const downloadPromise = session.streamTo(destination)
            .then(() => {
                logger.info(`SDK streamTo finished successfully. Closing writable stream.`);
                writable.close();
            })
            .catch((err) => {
                logger.error("SDK streamTo failed:", err);
                writable.abort(err).catch(e => logger.error(e));
            });

        event.waitUntil(downloadPromise);

        logger.debug(`Returning 200 OK Response`);
        return new Response(readable, {
            status: 200,
            headers
        });

    } catch (err) {
        logger.error("Fatal Error handling download request:", err);
        return createErrorResponse(err);
    }
}
