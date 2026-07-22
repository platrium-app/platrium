import type { MetadataRoute } from 'next'
import { source } from '@/lib/source'

export const dynamic = 'force-static';

export default function sitemap(): MetadataRoute.Sitemap {
    const baseUrl = 'https://docs.platrium.org'
    const docsPages = source.getPages().map((page) => {
        // Cast data to access lastModified safely across content source types
        const pageData = page.data as { lastModified?: Date | string | number };
        if (!pageData.lastModified)
            pageData.lastModified = new Date()

        return {
            url: `${baseUrl}${page.url}`,
            lastModified: new Date(pageData.lastModified),
            changeFrequency: 'weekly' as const,
            priority: (page.url == "/") ? 1.0 : 0.8,
        }
    })

    return docsPages
}