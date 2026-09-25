import { useEffect, useState } from "react";

export interface ServiceWorkerStatus {
  isSupported: boolean;
  isReady: boolean;
  isControlling: boolean;
  hasError: boolean;
}

/**
 * Custom hook to monitor Service Worker readiness and trigger an automatic reload
 * once the Service Worker becomes active and controlling the page.
 */
export function useServiceWorkerReady(autoReloadOnReady = true): ServiceWorkerStatus {
  const [status, setStatus] = useState<ServiceWorkerStatus>(() => {
    const isSupported = typeof window !== "undefined" && "serviceWorker" in navigator;
    const isControlling = isSupported && !!navigator.serviceWorker.controller;
    return {
      isSupported,
      isReady: isControlling,
      isControlling,
      hasError: !isSupported,
    };
  });

  useEffect(() => {
    if (!status.isSupported) return;

    // If SW is already controlling the client, we are ready
    if (navigator.serviceWorker.controller) {
      setStatus({
        isSupported: true,
        isReady: true,
        isControlling: true,
        hasError: false,
      });
      return;
    }

    let isMounted = true;

    const onControllerChange = () => {
      if (!isMounted) return;
      if (autoReloadOnReady) {
        window.location.reload();
      } else {
        setStatus({
          isSupported: true,
          isReady: true,
          isControlling: true,
          hasError: false,
        });
      }
    };

    navigator.serviceWorker.addEventListener("controllerchange", onControllerChange);

    // Wait for SW ready resolution
    navigator.serviceWorker.ready
      .then(() => {
        if (!isMounted) return;
        if (autoReloadOnReady && !navigator.serviceWorker.controller) {
          setTimeout(() => {
            window.location.reload();
          }, 300);
        } else {
          setStatus({
            isSupported: true,
            isReady: true,
            isControlling: !!navigator.serviceWorker.controller,
            hasError: false,
          });
        }
      })
      .catch(() => {
        if (isMounted) {
          setStatus((prev) => ({ ...prev, hasError: true }));
        }
      });

    return () => {
      isMounted = false;
      navigator.serviceWorker.removeEventListener("controllerchange", onControllerChange);
    };
  }, [status.isSupported, autoReloadOnReady]);

  return status;
}
