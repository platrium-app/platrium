import { createGraphQL } from '@fumadocs/graphql/server';
import path from 'path';
import fs from 'fs';

const graphqlDir = path.join(process.cwd(), '../api/graphql');

function enumerateGQLDefinitions(dir: string): string[] {
  let results: string[] = [];
  const list = fs.readdirSync(dir);
  for (const file of list) {
    const fullPath = path.join(dir, file);
    const stat = fs.statSync(fullPath);
    if (stat && stat.isDirectory()) {
      results = results.concat(enumerateGQLDefinitions(fullPath));
    } else if (fullPath.endsWith('.graphql')) {
      results.push(fullPath);
    }
  }
  return results;
}

const files = enumerateGQLDefinitions(graphqlDir);

export const graphql = createGraphQL({
  // the GraphQL schema, it accepts:
  // SDL files/URLs (including `extend type`), SDL text,
  // introspection results, and `GraphQLSchema` instances.
  input: {
    platrium: files,
  },
});