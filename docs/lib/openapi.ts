import { createOpenAPI } from 'fumadocs-openapi/server';

// note: this is a server-side API
export const openapi = createOpenAPI({
    input: ['../api/rest/_generated/openapi.yaml'],
});