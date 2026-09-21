// This file manages the configuration and endpoint construction for backend services.
// In the future, this can be expanded to support multiple backends, user-defined URLs, etc.

export const DEFAULT_BACKEND_URL = "http://localhost:3000";

/**
 * Returns the GraphQL endpoint URL for the specified base URL.
 */
export function getGraphQLEndpoint(baseUrl: string = DEFAULT_BACKEND_URL): string {
  // Ensure we don't have double slashes if baseUrl has a trailing slash
  const cleanBase = baseUrl.replace(/\/+$/, "");
  return `${cleanBase}/graphql`;
}
