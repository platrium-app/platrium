import React, { useEffect, useRef, useState } from 'react';
import * as pdfjsLib from 'pdfjs-dist';

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
      <canvas
        ref={canvasRef}
        style={{
          display: isRendered ? 'block' : 'none',
          backgroundColor: 'white',
          boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
        }}
      />
    </div>
  );
};
