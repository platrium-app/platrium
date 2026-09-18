import { ApolloClient, InMemoryCache, HttpLink } from "@apollo/client";
import { getGraphQLEndpoint } from "../config/backends";

export const client = new ApolloClient({
  link: new HttpLink({
    uri: getGraphQLEndpoint(),
    credentials: "include", // Ensures HTTP-only session cookies are sent
  }),
  cache: new InMemoryCache(),
});
