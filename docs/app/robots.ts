import type { MetadataRoute } from 'next'

export const dynamic = 'force-static';

export default function robots(): MetadataRoute.Robots {
    return {
        sitemap: 'https://docs.platrium.org/sitemap.xml',
        rules: {
            userAgent: '*',
            allow: '/',
        },
    }
}