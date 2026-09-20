'use client';
import { createOpenAPIPage } from 'fumadocs-openapi/ui';
import { createGraphQLPage } from '@fumadocs/graphql/ui';

export const OpenAPIPage = createOpenAPIPage();
export const GraphQLPage = createGraphQLPage({
    playground: {
        // enable the interactive playground on operation pages (optional)
        //url: 'https://api.example.com/graphql',
    },
});