/// <reference lib="webworker" />

import initWasm from 'platrium-sdk';
import { handleDownloadRequest } from './downloads';
import { createLogger } from '../lib/logging';

const logger = createLogger("Platrium SW");

declare const self: ServiceWorkerGlobalScope;

// Keep track of whether WASM is initialized globally for the Service Worker
let wasmInitialized = false;
async function ensureWasmInit() {
    if (!wasmInitialized) {
        await initWasm();
        wasmInitialized = true;
    }
}

self.addEventListener('install', () => {
    logger.info("Service Worker installing...");
    // Skip waiting to immediately activate the worker
    self.skipWaiting();
});

self.addEventListener('activate', (event) => {
    logger.info("Service Worker activating...");
    // Take control of all clients immediately
    event.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (event) => {
    event.respondWith((async () => {
        const url = new URL(event.request.url);

        // Ensure SDK is initialized before handling any requests
        await ensureWasmInit();

        // Route download interception requests
        const downloadMatch = url.pathname.match(/^\/rawcontent\/([a-zA-Z0-9-]+)$/);
        if (downloadMatch) {
            const fileId = downloadMatch[1];
            return handleDownloadRequest(event, fileId, url);
        }

        // Future offline page routing can go here

        // Fallback to network
        return fetch(event.request);
    })());
});
