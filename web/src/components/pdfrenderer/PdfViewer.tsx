import React, { useRef, useEffect } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { usePdfDocument } from './usePdfDocument';
import { PdfPageCanvas } from './PdfPageCanvas';
import { Skeleton } from '../ui/skeleton';
import { Button } from '../ui/button';
import { Slider } from '../ui/slider';
import { ZoomIn, ZoomOut, AlertTriangle, ChevronLeft, ChevronRight, PanelLeft } from 'lucide-react';
import { PlaceholderView } from '@/components/custom/PlaceholderView';
import { SidebarProvider, useSidebar } from '@/components/ui/sidebar';
import { PdfViewerSidebar } from './PdfViewerSidebar';

interface PdfRendererProps {
  fileUrl: string;
  zoomLevel: number;
  setZoomLevel: React.Dispatch<React.SetStateAction<number>>;
  orientation: number;
  fitMode: "width" | "page" | "auto" | null;
  setFitMode: React.Dispatch<React.SetStateAction<"width" | "page" | "auto" | null>>;
}

const SidebarToggleButton = () => {
  const { toggleSidebar } = useSidebar();
  return (
    <Button
      variant="ghost"
      size="icon"
      className="h-8 w-8 text-muted-foreground"
      onClick={toggleSidebar}
    >
      <PanelLeft className="w-4 h-4" />
    </Button>
  );
};

export const PdfRenderer: React.FC<PdfRendererProps> = ({ 
  fileUrl,
  zoomLevel,
  setZoomLevel,
  orientation,
  fitMode,
  setFitMode,
}) => {
  const { doc, firstPageDims, loading, error } = usePdfDocument(fileUrl);

  const parentRef = useRef<HTMLDivElement>(null);

  // Handle fit modes dynamically
  useEffect(() => {
    if (fitMode && firstPageDims && parentRef.current) {
      const container = parentRef.current;
      if (fitMode === 'width') {
        // Leave 32px total margin
        const scale = (container.clientWidth - 32) / firstPageDims.w;
        setZoomLevel(Number(scale.toFixed(2)));
      } else if (fitMode === 'auto') {
        // Leave 128px total margin (64px each side) for an elegant default fit
        const scale = (container.clientWidth - 128) / firstPageDims.w;
        setZoomLevel(Number(scale.toFixed(2)));
      } else if (fitMode === 'page') {
        const scaleH = (container.clientHeight - 32) / firstPageDims.h;
        const scaleW = (container.clientWidth - 32) / firstPageDims.w;
        setZoomLevel(Number(Math.min(scaleH, scaleW).toFixed(2)));
      }
    }
  }, [fitMode, firstPageDims, setZoomLevel]);

  // The virtualizer handles the vertical scrolling layout
  const rowVirtualizer = useVirtualizer({
    count: doc?.numPages || 0,
    getScrollElement: () => parentRef.current,
    estimateSize: () => (848 * zoomLevel) + 16, // Default A4 height + 16px margin
    overscan: 3, // render 3 pages before/after the visible area for smooth scrolling
  });

  // Re-measure when zoom changes
  useEffect(() => {
    rowVirtualizer.measure();
  }, [zoomLevel, rowVirtualizer]);

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center w-full h-full p-8 space-y-4">
        <Skeleton className="w-[800px] h-[1000px] rounded-lg" />
      </div>
    );
  }

  if (error || !doc) {
    return (
      <PlaceholderView
        icon={AlertTriangle}
        title="Failed to load PDF"
        description={error?.message || "An unknown error occurred while loading the document."}
      />
    );
  }

  const virtualItems = rowVirtualizer.getVirtualItems();
  let currentIdx = 1;
  if (parentRef.current && virtualItems.length > 0) {
    const containerCenter = parentRef.current.scrollTop + parentRef.current.clientHeight / 2;
    const activeItem = virtualItems.find(
      (item) => containerCenter >= item.start && containerCenter <= item.start + item.size
    );
    currentIdx = activeItem ? activeItem.index + 1 : virtualItems[0].index + 1;
  } else if (virtualItems.length > 0) {
    currentIdx = virtualItems[0].index + 1;
  }

  return (
    <SidebarProvider defaultOpen={true} className="w-full h-full bg-background">
      <PdfViewerSidebar 
        doc={doc} 
        onNavigate={(idx) => rowVirtualizer.scrollToIndex(idx, { align: 'start' })} 
      />
      <div className="flex flex-col w-full h-full bg-muted/30 overflow-hidden">
        {/* Sleek Toolbar */}
        <div className="flex items-center justify-between w-full h-12 px-4 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 z-10 shrink-0">

        {/* Left: Sidebar Toggle */}
        <div className="flex items-center w-1/3">
          <SidebarToggleButton />
        </div>

        {/* Center: Page Controls */}
        <div className="flex items-center justify-center space-x-1 w-1/3">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={currentIdx <= 1}
            onClick={() => rowVirtualizer.scrollToIndex(currentIdx - 2, { align: 'start' })}
          >
            <ChevronLeft className="w-4 h-4" />
          </Button>
          <span className="text-sm font-medium tabular-nums w-16 text-center">
            {currentIdx} / {doc?.numPages || 0}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={!doc || currentIdx >= doc.numPages}
            onClick={() => rowVirtualizer.scrollToIndex(currentIdx, { align: 'start' })}
          >
            <ChevronRight className="w-4 h-4" />
          </Button>
        </div>

        {/* Right: Zoom Controls */}
        <div className="flex items-center justify-end space-x-2 w-1/3">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setZoomLevel((z) => Math.max(0.2, z - 0.2))}
          >
            <ZoomOut className="w-4 h-4" />
          </Button>
          <div className="w-32 hidden sm:block">
            <Slider
              value={[zoomLevel]}
              min={0.2}
              max={3.0}
              step={0.1}
              className="cursor-pointer"
              onValueChange={(val) => {
                setZoomLevel(Array.isArray(val) ? val[0] : (val as unknown as number[])[0] || val as unknown as number);
                setFitMode(null);
              }}
            />
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setZoomLevel((z) => Math.min(3.0, z + 0.2))}
          >
            <ZoomIn className="w-4 h-4" />
          </Button>
          <div className="text-sm text-muted-foreground w-12 text-right tabular-nums">
            {Math.round(zoomLevel * 100)}%
          </div>
        </div>
      </div>

      {/* Virtualized scroll container */}
      <div
        ref={parentRef}
        className="flex-1 w-full overflow-y-auto"
        style={{
          contain: 'strict',
        }}
      >
        <div
          style={{
            height: `${rowVirtualizer.getTotalSize()}px`,
            width: '100%',
            position: 'relative',
          }}
        >
          {rowVirtualizer.getVirtualItems().map((virtualRow) => {
            return (
              <div
                key={virtualRow.index}
                data-index={virtualRow.index}
                ref={rowVirtualizer.measureElement}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${virtualRow.start}px)`,
                  display: 'flex',
                  justifyContent: 'center',
                }}
              >
                <PdfPageCanvas
                  doc={doc}
                  pageIndex={virtualRow.index + 1} // pdfjs is 1-indexed
                  scale={zoomLevel}
                  orientation={orientation}
                />
              </div>
            );
          })}
        </div>
      </div>
    </div>
    </SidebarProvider>
  );
};
