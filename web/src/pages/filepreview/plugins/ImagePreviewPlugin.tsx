import React, { useState, useRef, useEffect, useCallback } from "react";
import type { FilePreviewPluginProps, PluginDefinition } from "../PluginDefinition";
import { constructRawContentUrl } from "@/lib/utils";
import { useRegisterMenu } from "../FilePreviewMenuContext";
import {
    ZoomIn,
    ZoomOut,
    Maximize2,
    RotateCw,
    RotateCcw,
    AlertTriangle,
} from "lucide-react";
import { PlaceholderView } from "@/components/custom/PlaceholderView";
import { Spinner } from "@/components/ui/spinner";

const MIN_ZOOM = 0.5;
const MAX_ZOOM = 5.0;

const isPointerOverImage = (
    imgEl: HTMLImageElement | null,
    clientX: number,
    clientY: number
): boolean => {
    if (!imgEl) return false;
    const rect = imgEl.getBoundingClientRect();
    if (
        clientX < rect.left ||
        clientX > rect.right ||
        clientY < rect.top ||
        clientY > rect.bottom
    ) {
        return false;
    }

    const { naturalWidth, naturalHeight } = imgEl;
    if (!naturalWidth || !naturalHeight || !rect.width || !rect.height) return true;

    const containerAspect = rect.width / rect.height;
    const imageAspect = naturalWidth / naturalHeight;

    let visualWidth = rect.width;
    let visualHeight = rect.height;

    if (imageAspect > containerAspect) {
        visualHeight = rect.width / imageAspect;
    } else {
        visualWidth = rect.height * imageAspect;
    }

    const padX = (rect.width - visualWidth) / 2;
    const padY = (rect.height - visualHeight) / 2;

    return (
        clientX >= rect.left + padX &&
        clientX <= rect.right - padX &&
        clientY >= rect.top + padY &&
        clientY <= rect.bottom - padY
    );
};

const ImagePreviewPluginComponent: React.FC<FilePreviewPluginProps> = ({ info }) => {
    const [rotation, setRotation] = useState(0);
    const [transformState, setTransformState] = useState({
        scale: 1.0,
        position: { x: 0, y: 0 },
    });
    const { scale, position } = transformState;

    const [isDragging, setIsDragging] = useState(false);
    const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
    const [isHoveringImage, setIsHoveringImage] = useState(false);
    const [hasError, setHasError] = useState(false);
    const [isLoading, setIsLoading] = useState(true);

    const containerRef = useRef<HTMLDivElement>(null);
    const imageRef = useRef<HTMLImageElement>(null);

    const handleZoomIn = useCallback(() => {
        setTransformState((prev) => {
            const nextScale = Math.min(MAX_ZOOM, Number((prev.scale * 1.25).toFixed(2)));
            const scaleRatio = nextScale / prev.scale;
            return {
                scale: nextScale,
                position: {
                    x: prev.position.x * scaleRatio,
                    y: prev.position.y * scaleRatio,
                },
            };
        });
    }, []);

    const handleZoomOut = useCallback(() => {
        setTransformState((prev) => {
            const nextScale = Math.max(MIN_ZOOM, Number((prev.scale / 1.25).toFixed(2)));
            if (nextScale <= 1.0) {
                return {
                    scale: nextScale,
                    position: { x: 0, y: 0 },
                };
            }
            const scaleRatio = nextScale / prev.scale;
            return {
                scale: nextScale,
                position: {
                    x: prev.position.x * scaleRatio,
                    y: prev.position.y * scaleRatio,
                },
            };
        });
    }, []);

    const handleResetZoom = useCallback(() => {
        setTransformState({
            scale: 1.0,
            position: { x: 0, y: 0 },
        });
    }, []);

    useRegisterMenu("View", [
        {
            id: "img-zoom-in",
            label: "Zoom In",
            category: "View",
            icon: ZoomIn,
            onClick: handleZoomIn,
        },
        {
            id: "img-zoom-out",
            label: "Zoom Out",
            category: "View",
            icon: ZoomOut,
            onClick: handleZoomOut,
        },
        {
            id: "img-zoom-reset",
            label: "Reset Zoom",
            category: "View",
            icon: Maximize2,
            onClick: handleResetZoom,
        },
        {
            id: "img-rotate-right",
            label: "Rotate Right",
            category: "View",
            icon: RotateCw,
            separatorBefore: true,
            onClick: () => setRotation((r) => r + 90),
        },
        {
            id: "img-rotate-left",
            label: "Rotate Left",
            category: "View",
            icon: RotateCcw,
            onClick: () => setRotation((r) => r - 90),
        },
    ]);

    const [isWheeling, setIsWheeling] = useState(false);
    const wheelTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Attach non-passive wheel event listener for smooth focal-point zooming
    useEffect(() => {
        const container = containerRef.current;
        if (!container) return;

        const handleWheel = (e: WheelEvent) => {
            // Only zoom when pointer is directly above rendered image
            if (!isPointerOverImage(imageRef.current, e.clientX, e.clientY)) {
                return;
            }

            e.preventDefault();

            setIsWheeling(true);
            if (wheelTimeoutRef.current) clearTimeout(wheelTimeoutRef.current);
            wheelTimeoutRef.current = setTimeout(() => setIsWheeling(false), 150);

            const rect = container.getBoundingClientRect();
            const mouseX = e.clientX - rect.left - rect.width / 2;
            const mouseY = e.clientY - rect.top - rect.height / 2;

            // Normalize deltaY using proportional exponential scaling (macOS trackpad & mouse wheel friendly)
            const zoomFactor = Math.pow(1.0015, -e.deltaY);

            setTransformState((prev) => {
                const nextScale = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, prev.scale * zoomFactor));
                if (nextScale <= 1.0) {
                    return {
                        scale: nextScale,
                        position: { x: 0, y: 0 },
                    };
                }

                const scaleRatio = nextScale / prev.scale;

                return {
                    scale: nextScale,
                    position: {
                        x: mouseX - (mouseX - prev.position.x) * scaleRatio,
                        y: mouseY - (mouseY - prev.position.y) * scaleRatio,
                    },
                };
            });
        };

        container.addEventListener("wheel", handleWheel, { passive: false });
        return () => {
            container.removeEventListener("wheel", handleWheel);
            if (wheelTimeoutRef.current) clearTimeout(wheelTimeoutRef.current);
        };
    }, []);

    // Drag / Pan event handlers
    const handleMouseDown = (e: React.MouseEvent) => {
        if (scale <= 1.0) return;
        if (!isPointerOverImage(imageRef.current, e.clientX, e.clientY)) return;

        e.preventDefault();
        setIsDragging(true);
        setDragStart({
            x: e.clientX - position.x,
            y: e.clientY - position.y,
        });
    };

    const handleMouseMove = (e: React.MouseEvent) => {
        const hovering = isPointerOverImage(imageRef.current, e.clientX, e.clientY);
        setIsHoveringImage(hovering);

        if (!isDragging || scale <= 1.0) return;
        setTransformState((prev) => ({
            ...prev,
            position: {
                x: e.clientX - dragStart.x,
                y: e.clientY - dragStart.y,
            },
        }));
    };

    const handleMouseUp = () => {
        setIsDragging(false);
    };

    if (hasError) {
        return (
            <PlaceholderView
                icon={AlertTriangle}
                title="Failed to Load Image"
                description={`Unable to render ${info.fileName}. The file may be corrupted or in an unsupported image format.`}
                variant="warning"
            />
        );
    }

    const imageUrl = constructRawContentUrl(info.fileId);
    const isZoomedIn = scale > 1.0;

    return (
        <div
            ref={containerRef}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={() => {
                handleMouseUp();
                setIsHoveringImage(false);
            }}
            className={`relative flex h-full w-full items-center justify-center p-4 overflow-hidden select-none ${isZoomedIn
                    ? isDragging
                        ? "cursor-grabbing"
                        : isHoveringImage
                            ? "cursor-grab"
                            : "cursor-default"
                    : "cursor-default"
                }`}
        >
            {isLoading && (
                <div className="absolute inset-0 flex items-center justify-center bg-background/50 z-10">
                    <Spinner className="h-8 w-8 text-muted-foreground" />
                </div>
            )}
            <img
                ref={imageRef}
                draggable={false}
                onDragStart={(e) => e.preventDefault()}
                src={imageUrl}
                alt={info.fileName}
                onLoad={() => setIsLoading(false)}
                onError={() => {
                    setIsLoading(false);
                    setHasError(true);
                }}
                style={{
                    transform: `translate3d(${position.x}px, ${position.y}px, 0px) scale(${scale}) rotate(${rotation}deg)`,
                    transition: isDragging || isWheeling ? "none" : "transform 150ms cubic-bezier(0.2, 0, 0, 1)",
                }}
                className={`max-h-full max-w-full object-contain ${isLoading ? "opacity-0" : "opacity-100"
                    }`}
            />
        </div>
    );
};

export const ImagePreviewPlugin: PluginDefinition = {
    id: "image-previewer",
    name: "Image Previewer",
    supportedMimeTypes: ["image/*"],
    maxSizeBytes: 25 * 1024 * 1024, // 25 MB limit for image previewing
    component: ImagePreviewPluginComponent,
};

export default ImagePreviewPlugin;
