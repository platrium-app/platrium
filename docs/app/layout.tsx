import './globals.css';
import { Metadata } from 'next';
import { RootProvider } from 'fumadocs-ui/provider/next';
import SearchDialog from '@/components/search';
import type { ReactNode } from 'react';

export const metadata: Metadata = {
  title: {
    template: '%s | Platrium Docs',
    default: 'Platrium Documentation',
  },
  description: "Explore Platrium's User Guides, Developer SDKs, Internal APIs, and deep open-source architecture.",
  keywords: ["Platrium", "Platrium REST API", "Platrium SDK", "Platrium Internal Design", "Platrium Concurrency", "Open Source File Storage", "Content Addressable Storage"]
};

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="flex flex-col min-h-screen">
        <RootProvider search={{
          SearchDialog,
        }}>{children}</RootProvider>
      </body>
    </html>
  );
}