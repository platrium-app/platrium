import { createGraphQL } from '@fumadocs/graphql/server';
import path from 'path';
import fs from 'fs';

const graphqlDir = path.join(process.cwd(), '../api/graphql');
const files = fs.readdirSync(graphqlDir)
  .filter(file => file.endsWith('.graphql'))
  .map(file => path.join(graphqlDir, file));

// note: this is a server-side API
export const graphql = createGraphQL({
    // the GraphQL schema, it accepts:
    // SDL files/URLs (including `extend type`), SDL text,
    // introspection results, and `GraphQLSchema` instances.
    input: files,
});