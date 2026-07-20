import { docs } from 'collections/server';
import { loader } from 'fumadocs-core/source';
import { icons } from 'lucide-react';
import { createElement } from 'react';
import { openapi } from '@/lib/openapi';

export const source = loader(
    {
        docs: docs.toFumadocsSource(),
        api: await openapi.staticSource({
            baseDir: 'api/rest',
        }),
    },
    {
        baseUrl: '/',
        plugins: [openapi.loaderPlugin()],
        icon(icon) {
            if (!icon) return;
            if (icon in icons) return createElement(icons[icon as keyof typeof icons]);
        },
    },
);