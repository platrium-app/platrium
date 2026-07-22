import { remarkDirectiveAdmonition, remarkMdxMermaid } from 'fumadocs-core/mdx-plugins';
import { defineDocs, defineConfig } from 'fumadocs-mdx/config';
import { transformerTwoslash } from 'fumadocs-twoslash';
import { rehypeCodeDefaultOptions } from 'fumadocs-core/mdx-plugins';
import remarkDirective from 'remark-directive';
import lastModified from 'fumadocs-mdx/plugins/last-modified';

export const docs = defineDocs({
    dir: 'content/docs',
});

export default defineConfig({
    plugins: [lastModified()],
    mdxOptions: {
        remarkPlugins: [remarkMdxMermaid, remarkDirective, remarkDirectiveAdmonition],
        rehypeCodeOptions: {
            transformers: [...(rehypeCodeDefaultOptions.transformers ?? []), transformerTwoslash()],
            themes: {
                light: 'github-light',
                dark: 'github-dark',
            },

            // important: Shiki doesn't support lazy loading languages for codeblocks in Twoslash popups
            // make sure to define them first (e.g. the common ones)
            langs: ['js', 'jsx', 'ts', 'tsx', 'rs', 'tsp', 'kt', 'java', 'go', 'swift', 'graphql'],
        },
    },
});