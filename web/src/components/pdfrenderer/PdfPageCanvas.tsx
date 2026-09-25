import React, { useEffect, useRef, useState } from 'react';
import * as pdfjsLib from 'pdfjs-dist';
import 'pdfjs-dist/web/pdf_viewer.css';

import { Spinner } from '../ui/spinner';

interface PdfPageCanvasProps {
  doc: pdfjsLib.PDFDocumentProxy;
  pageIndex: number; // 1-indexed
  scale: number;
  orientation: number;
}

export const PdfPageCanvas: React.FC<PdfPageCanvasProps> = ({
  doc,
  pageIndex,
  scale,
  orientation: rotation,
}) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const textLayerRef = useRef<HTMLDivElement>(null);
  const annotationLayerRef = useRef<HTMLDivElement>(null);
  const renderTaskRef = useRef<pdfjsLib.RenderTask | null>(null);
  const [isRendered, setIsRendered] = useState(false);
  const [pageDims, setPageDims] = useState<{ w: number; h: number } | null>(null);

  useEffect(() => {
    let active = true;

    const renderPage = async () => {
      const canvas = canvasRef.current;
      if (!canvas) return;

      setIsRendered(false);

      try {
        const page = await doc.getPage(pageIndex);
        if (!active) return;

        // Get viewport at the required zoom scale and rotation
        const viewport = page.getViewport({ scale, rotation });

        if (active) {
          setPageDims({ w: viewport.width, h: viewport.height });
        }

        // Support high-DPI displays (retina screens)
        const outputScale = window.devicePixelRatio || 1;

        canvas.width = Math.floor(viewport.width * outputScale);
        canvas.height = Math.floor(viewport.height * outputScale);
        canvas.style.width = Math.floor(viewport.width) + 'px';
        canvas.style.height = Math.floor(viewport.height) + 'px';

        const transform =
          outputScale !== 1 ? [outputScale, 0, 0, outputScale, 0, 0] : undefined;

        const renderContext: any = {
          canvasContext: canvas.getContext('2d')!,
          transform,
          viewport,
        };

        const renderTask = page.render(renderContext);
        renderTaskRef.current = renderTask;

        await renderTask.promise;

        if (active) {
          setIsRendered(true);

          if (textLayerRef.current) {
            if (!active) return;
            textLayerRef.current.innerHTML = '';
            const textLayer = new pdfjsLib.TextLayer({
              textContentSource: page.streamTextContent({ includeMarkedContent: true }),
              container: textLayerRef.current,
              viewport: viewport,
            });
            await textLayer.render();

            // Auto-link plain text URLs in the text layer
            const spans = textLayerRef.current.querySelectorAll('span');
            spans.forEach(span => {
              const text = span.textContent;
              if (text && text.includes('http')) {
                const urlMatch = text.match(/(https?:\/\/[^\s]+)/);
                if (urlMatch) {
                  const url = urlMatch[0];
                  // Replace text with a hyperlink
                  span.innerHTML = text.replace(
                    url,
                    `<a href="${url}" target="_blank" rel="noopener noreferrer" style="pointer-events: auto; text-decoration: underline;">${url}</a>`
                  );
                  // Ensure the span itself doesn't block the click
                  span.style.pointerEvents = 'auto';
                }
              }
            });
          }

          if (annotationLayerRef.current) {
            const annotations = await page.getAnnotations({ intent: 'display' });
            if (!active) return;
            annotationLayerRef.current.innerHTML = '';

            const pdfLinkService = {
              getDestinationHash: (dest: any) => JSON.stringify(dest),
              getAnchorUrl: (url: string) => url || '',
              setDocument: () => { },
              executeNamedAction: () => { },
              cachePageRef: () => { },
              isPageVisible: () => true,
              isPageCached: () => true,
              navigateTo: async (dest: any) => {
                try {
                  let explicitDest = dest;
                  if (typeof dest === 'string') {
                    explicitDest = await doc.getDestination(dest);
                  }
                  if (Array.isArray(explicitDest)) {
                    const pageIndex = await doc.getPageIndex(explicitDest[0]);
                    window.dispatchEvent(new CustomEvent('pdfrenderer_jumptopage', { detail: { pageIndex } }));
                  }
                } catch (e) {
                  console.warn("Failed to navigate to destination", e);
                }
              },
            } as any;

            const annotationLayer = new pdfjsLib.AnnotationLayer({
              page,
              viewport,
              div: annotationLayerRef.current,
              accessibilityManager: null,
              annotationCanvasMap: null,
              annotationEditorUIManager: null,
              structTreeLayer: null,
              commentManager: null,
              annotationStorage: null,
              linkService: pdfLinkService,
            });

            await annotationLayer.render({
              annotations,
              viewport,
              div: annotationLayerRef.current,
              page,
              imageResourcesPath: '',
              renderForms: false,
              linkService: pdfLinkService,
            });

            // Intercept external links to open in a new tab
            const clickHandler = (e: MouseEvent) => {
              const target = e.target as HTMLElement;
              const anchor = target.closest('a');
              if (anchor && anchor.href && !anchor.href.startsWith(window.location.origin) && !anchor.href.includes('#')) {
                e.preventDefault();
                window.open(anchor.href, '_blank', 'noopener,noreferrer');
              }
            };
            annotationLayerRef.current.addEventListener('click', clickHandler);
            
            // Clean up handler if the effect re-runs
            if ((annotationLayerRef.current as any)._clickHandler) {
              annotationLayerRef.current.removeEventListener('click', (annotationLayerRef.current as any)._clickHandler);
            }
            (annotationLayerRef.current as any)._clickHandler = clickHandler;
          }
        }
      } catch (err: any) {
        if (err.name === 'RenderingCancelledException') {
          // Normal cancellation, no need to log
        } else {
          console.error(`Error rendering page ${pageIndex}:`, err);
        }
      } finally {
        renderTaskRef.current = null;
      }
    };

    renderPage();

    return () => {
      active = false;
      if (renderTaskRef.current) {
        renderTaskRef.current.cancel();
      }
    };
  }, [doc, pageIndex, scale, rotation]);

  // Default estimates if not loaded yet (assumes standard ~600px width A4 at scale 1)
  const renderWidth = pageDims?.w || 600 * scale;
  const renderHeight = pageDims?.h || 848 * scale;

  return (
    <div
      style={{
        width: '100%',
        height: renderHeight,
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        marginBottom: '1rem',
      }}
    >
      {!isRendered && (
        <div
          className="absolute flex items-center justify-center bg-white"
          style={{
            width: renderWidth,
            height: renderHeight,
            boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
          }}
        >
          <Spinner className="w-8 h-8" />
        </div>
      )}
      <div className="relative" style={{ width: renderWidth, height: renderHeight }}>
        <canvas
          ref={canvasRef}
          style={{
            display: isRendered ? 'block' : 'none',
            backgroundColor: 'white',
            boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
            width: renderWidth,
            height: renderHeight,
          }}
        />
        <div
          ref={textLayerRef}
          className="textLayer"
          style={{ width: renderWidth, height: renderHeight, left: 0, top: 0, position: 'absolute', zIndex: 1, '--scale-factor': scale } as React.CSSProperties}
        />
        <div
          ref={annotationLayerRef}
          className="annotationLayer"
          style={{ width: renderWidth, height: renderHeight, left: 0, top: 0, position: 'absolute', zIndex: 2, pointerEvents: 'none', '--scale-factor': scale } as React.CSSProperties}
        />
      </div>
    </div>
  );
};
