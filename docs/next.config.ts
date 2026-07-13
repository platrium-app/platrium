import { createMDX } from 'fumadocs-mdx/next';
import type { NextConfig } from 'next';

const withMDX = createMDX();

const nextConfig: NextConfig = {
  output: 'export',
  reactStrictMode: true,
  serverExternalPackages: ['typescript', 'twoslash'],
};

export default withMDX(nextConfig);