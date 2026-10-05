// This file manages the configuration and endpoint construction for backend services.
// In the future, this can be expanded to support multiple backends, user-defined URLs, etc.

/**
 * The web UI is served by the engine it talks to, so by default every endpoint is
 * relative to the page's own origin. A hard-coded host such as localhost breaks as
 * soon as the page is opened from another device: there, "localhost" is that device.
 */
export const DEFAULT_BACKEND_URL = "";

/**
 * Returns the GraphQL endpoint URL for the specified base URL (same origin by default).
 */
export function getGraphQLEndpoint(baseUrl: string = DEFAULT_BACKEND_URL): string {
  // Ensure we don't have double slashes if baseUrl has a trailing slash
  const cleanBase = baseUrl.replace(/\/+$/, "");
  return `${cleanBase}/graphql`;
}

/**
 * Returns the GraphQL WebSocket endpoint URL for the specified base URL (same origin by default).
 */
export function getGraphQLWSEndpoint(baseUrl: string = DEFAULT_BACKEND_URL): string {
  const cleanBase = baseUrl.replace(/\/+$/, "");
  if (!cleanBase) {
    // WebSocket URLs must be absolute.
    const scheme = window.location.protocol === "https:" ? "wss" : "ws";
    return `${scheme}://${window.location.host}/graphql`;
  }
  return `${cleanBase.replace(/^http/, "ws")}/graphql`;
}
