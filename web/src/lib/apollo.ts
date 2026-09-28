import { ApolloClient, InMemoryCache, HttpLink, ApolloLink } from "@apollo/client";
import { getMainDefinition } from "@apollo/client/utilities";
import { GraphQLWsLink } from "@apollo/client/link/subscriptions";
import { createClient } from "graphql-ws";
import { getGraphQLEndpoint, getGraphQLWSEndpoint } from "../config/backends";

const httpLink = new HttpLink({
  uri: getGraphQLEndpoint(),
  credentials: "include", // Ensures HTTP-only session cookies are sent
});

const wsLink = new GraphQLWsLink(
  createClient({
    url: getGraphQLWSEndpoint(),
    // Optional: connectionParams for auth if needed later
  })
);

// The split function takes three parameters:
// * A function that's called for each operation to execute
// * The Link to use for an operation if the function returns a "truthy" value
// * The Link to use for an operation if the function returns a "falsy" value
const splitLink = ApolloLink.split(
  ({ query }) => {
    const definition = getMainDefinition(query);
    return (
      definition.kind === "OperationDefinition" &&
      definition.operation === "subscription"
    );
  },
  wsLink,
  httpLink
);

export const client = new ApolloClient({
  link: splitLink,
  cache: new InMemoryCache(),
});
