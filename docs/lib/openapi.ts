import { createOpenAPI } from 'fumadocs-openapi/server';

// note: this is a server-side API
export const openapi = createOpenAPI({
    // We use Enterprise APIs by Default for Docs.
    input: ['../api/rest/_generated/openapi_ee.yaml'],
});