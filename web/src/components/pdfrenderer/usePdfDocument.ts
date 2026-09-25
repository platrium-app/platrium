import { useState, useEffect } from 'react';
import * as pdfjsLib from 'pdfjs-dist';
// Vite standard way to load the worker file cleanly
import pdfWorker from 'pdfjs-dist/build/pdf.worker.mjs?url';

// Initialize the worker globally once
pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorker;

export function usePdfDocument(fileUrl: string) {
  const [doc, setDoc] = useState<pdfjsLib.PDFDocumentProxy | null>(null);
  const [firstPageDims, setFirstPageDims] = useState<{w: number, h: number} | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let active = true;
    const loadPdf = async () => {
      try {
        setLoading(true);
        const loadingTask = pdfjsLib.getDocument({ url: fileUrl });
        const loadedDoc = await loadingTask.promise;
        
        if (!active) return;
        setDoc(loadedDoc);

        // Fetch just the first page to get native aspect ratio/dimensions
        if (loadedDoc.numPages > 0) {
          const page1 = await loadedDoc.getPage(1);
          const viewport = page1.getViewport({ scale: 1.0 });
          if (active) {
            setFirstPageDims({ w: viewport.width, h: viewport.height });
          }
          page1.cleanup();
        }
      } catch (err) {
        if (!active) return;
        setError(err instanceof Error ? err : new Error(String(err)));
      } finally {
        if (active) setLoading(false);
      }
    };

    loadPdf();

    // We do NOT destroy the doc on unmount here because we want to preserve it if it remounts quickly,
    // but in a strict viewer we might want to destroy it. Let's rely on the GC or handle it in the viewer.
    return () => {
      active = false;
    };
  }, [fileUrl]);

  return { doc, firstPageDims, loading, error };
}
