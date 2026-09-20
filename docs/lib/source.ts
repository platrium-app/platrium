import { docs } from 'collections/server';
import { loader } from 'fumadocs-core/source';
import { icons } from 'lucide-react';
import { createElement, type ComponentProps } from 'react';
import { openapi } from '@/lib/openapi';
import GraphQLIcon from '@/icons/graphql'
import { graphql } from '@/lib/graphql';

const customIcons: Record<string, React.ComponentType<{ className?: string }>> = {
    graphql: GraphQLIcon,
};

export const source = loader(
    {
        docs: docs.toFumadocsSource(),
        api: await openapi.staticSource({
            baseDir: 'api/rest',
        }),
        graphql: await graphql.staticSource({
            baseDir: 'api/graphql',
        }),
    },
    {
        baseUrl: '/',
        plugins: [openapi.loaderPlugin(), graphql.loaderPlugin()],
        icon(icon) {
            if (!icon) return;

            // Check custom icon registry first
            if (icon in customIcons) {
                return createElement(customIcons[icon]);
            }

            // Fallback to Lucide icons
            if (icon in icons) {
                return createElement(icons[icon as keyof typeof icons]);
            }
        },
    },
);