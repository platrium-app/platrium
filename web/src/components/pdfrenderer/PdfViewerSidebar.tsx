import React, { useEffect, useState } from 'react';
import * as pdfjsLib from 'pdfjs-dist';
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  useSidebar,
} from '@/components/ui/sidebar';
import { FileText, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';

interface PdfViewerSidebarProps {
  doc: pdfjsLib.PDFDocumentProxy | null;
  onNavigate: (pageIndex: number) => void;
}

export const PdfViewerSidebar: React.FC<PdfViewerSidebarProps> = ({ doc, onNavigate }) => {
  const { state } = useSidebar();
  const [outline, setOutline] = useState<any[] | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!doc) return;
    let active = true;
    doc.getOutline().then((outlineData) => {
      if (active) {
        setOutline(outlineData);
        setLoading(false);
      }
    }).catch(() => {
      if (active) {
        setOutline(null);
        setLoading(false);
      }
    });
    return () => {
      active = false;
    };
  }, [doc]);

  const handleNavigate = async (dest: any) => {
    if (!doc || !dest) return;
    try {
      let actualDest = dest;
      if (typeof dest === 'string') {
        actualDest = await doc.getDestination(dest);
      }
      if (actualDest && Array.isArray(actualDest)) {
        const pageRef = actualDest[0];
        // getPageIndex handles references natively
        const pageIndex = await doc.getPageIndex(pageRef);
        onNavigate(pageIndex);
      }
    } catch (e) {
      console.error("Failed to navigate to outline dest", e);
    }
  };

  const renderOutlineItems = (items: any[], depth = 0) => {
    return items.map((item, idx) => (
      <React.Fragment key={`${depth}-${idx}`}>
        <SidebarMenuItem>
          <SidebarMenuButton 
            onClick={() => handleNavigate(item.dest)}
            className="text-sm py-1 h-auto min-h-8"
            style={{ paddingLeft: `${depth * 1 + 0.5}rem` }}
            title={item.title}
          >
            <span className="line-clamp-2 leading-snug">{item.title}</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
        {item.items && item.items.length > 0 && renderOutlineItems(item.items, depth + 1)}
      </React.Fragment>
    ));
  };

  return (
    <Sidebar 
      collapsible="none" 
      className={cn(
        "bg-background shrink-0 h-full overflow-hidden transition-[width,border] duration-300 ease-in-out flex flex-col",
        state === "expanded" ? "w-64 border-r border-border/50" : "w-0 border-r-0"
      )}
    >
      <div className="w-64 flex flex-col h-full shrink-0">
        <SidebarHeader className="border-b border-border/50 px-4 flex items-center justify-center shrink-0 h-12">
          <h2 className="text-sm font-semibold tracking-tight text-foreground w-full">Table of Contents</h2>
        </SidebarHeader>
        <SidebarContent className="bg-background">
        {loading ? (
          <div className="flex items-center justify-center p-8 mt-12 text-muted-foreground">
            <Loader2 className="w-6 h-6 animate-spin" />
          </div>
        ) : outline && outline.length > 0 ? (
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                {renderOutlineItems(outline)}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ) : (
          <div className="flex flex-col items-center justify-center p-8 mt-12 text-center space-y-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
              <FileText className="h-6 w-6 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium text-foreground">No table of contents</p>
            <p className="text-xs text-muted-foreground max-w-[180px]">
              This document doesn't have an embedded outline or bookmarks.
            </p>
          </div>
        )}
      </SidebarContent>
      </div>
    </Sidebar>
  );
};
