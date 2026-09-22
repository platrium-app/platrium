import React, { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import { previewRegistry } from "./PluginRegistry";
import { DEFAULT_MAX_PREVIEW_SIZE_BYTES, type FilePreviewInfo, type PreviewFeatures } from "./PluginDefinition";
import { FallbackPlugin } from "./plugins/FallbackPlugin";
import { FilePreviewMenuBar } from "./FilePreviewMenuBar";
import { Spinner } from "@/components/ui/spinner";
import { PlaceholderView } from "@/components/custom/PlaceholderView";
import { CloudAlertIcon, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { constructRawContentUrl, parseFilenameFromContentDisposition } from "@/lib/utils";

import { FilePreviewMenuProvider } from "./FilePreviewMenuContext";

interface FilePreviewCoreProps {
    fileId: string;
    isModal?: boolean;
    onClose?: () => void;
}

interface FileMetadata {
    name: string;
    mimeType: string;
    sizeBytes?: number;
}

const FilePreviewInner: React.FC<FilePreviewCoreProps> = ({ fileId, isModal, onClose }) => {
    const [metadata, setMetadata] = useState<FileMetadata | null>(null);
    const [error, setError] = useState<Error | null>(null);
    const [features, setFeatures] = useState<PreviewFeatures>({});

    useEffect(() => {
        const resolveMetadata = async () => {
            try {
                const url = constructRawContentUrl(fileId);
                const response = await fetch(url, { method: "HEAD" });

                if (!response.ok) {
                    throw new Error(`HTTP error ${response.status}: ${response.statusText}`);
                }

                const mimeType = response.headers.get("content-type") || "application/unknown";
                const disposition = response.headers.get("content-disposition");
                const contentLength = response.headers.get("content-length");
                const fileName = parseFilenameFromContentDisposition(disposition, fileId);
                const sizeBytes = contentLength ? parseInt(contentLength, 10) : undefined;

                setMetadata({
                    name: fileName,
                    mimeType,
                    sizeBytes,
                });
            } catch (err) {
                setError(err instanceof Error ? err : new Error("Failed to load file metadata"));
            }
        };

        resolveMetadata();
    }, [fileId]);

    const renderPlaceholder = (
        icon: React.ComponentType<{ className?: string }>,
        title: string,
        description: React.ReactNode,
        variant: "ghost" | "warning" | "error"
    ) => (
        <div className={isModal ? "flex h-full w-full items-center justify-center bg-background/40 backdrop-blur-md relative" : "flex h-screen w-screen items-center justify-center bg-background/40 backdrop-blur-md relative"}>
            {isModal && onClose && (
                <div className="absolute top-3 right-3 z-50">
                    <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose} title="Close Preview">
                        <X className="h-4 w-4" />
                    </Button>
                </div>
            )}
            <PlaceholderView
                icon={icon}
                title={title}
                description={description}
                variant={variant}
            />
        </div>
    );

    if (error) {
        return renderPlaceholder(
            CloudAlertIcon,
            "Failed to Fetch File",
            `The following Error Occured: ${error.message}`,
            "error"
        );
    }

    if (!metadata) {
        return renderPlaceholder(
            Spinner,
            "Hang tight!",
            "We're fetching the information about the file.",
            "ghost"
        );
    }

    const pluginDefinition = previewRegistry.getPluginForMimeType(metadata.mimeType);

    // Calculate effective max size: undefined defaults to 32MB, 0 means unlimited
    const effectiveMaxSize = pluginDefinition.maxSizeBytes !== undefined
        ? pluginDefinition.maxSizeBytes
        : DEFAULT_MAX_PREVIEW_SIZE_BYTES;

    const isTooLarge = effectiveMaxSize > 0 && metadata.sizeBytes !== undefined && metadata.sizeBytes > effectiveMaxSize;

    const fileInfo: FilePreviewInfo = {
        fileId,
        fileName: metadata.name,
        mimeType: metadata.mimeType,
        sizeBytes: metadata.sizeBytes,
    };

    const containerClasses = isModal
        ? "flex flex-col h-full w-full bg-background/40 backdrop-blur-md text-foreground overflow-hidden relative"
        : "flex flex-col h-screen w-screen bg-background/40 backdrop-blur-md text-foreground overflow-hidden relative";

    const PluginComponent = pluginDefinition.component;
    const FallbackComponent = FallbackPlugin.component;

    const maxMb = (effectiveMaxSize / (1024 * 1024)).toFixed(0);
    const fileMb = metadata.sizeBytes ? (metadata.sizeBytes / (1024 * 1024)).toFixed(1) : undefined;

    return (
        <div className={containerClasses}>
            {/* Top Toolbar using FilePreviewMenuBar */}
            <FilePreviewMenuBar
                info={fileInfo}
                features={features}
                isModal={isModal}
                onClose={onClose}
            />

            {/* Content Area */}
            <div className="flex-1 flex flex-col min-h-0 w-full overflow-hidden bg-muted/10 backdrop-blur-sm relative items-center justify-center">
                {isTooLarge ? (
                    <FallbackComponent
                        info={fileInfo}
                        registerFeatures={setFeatures}
                        title="File Too Large to Preview"
                        description={`This file${fileMb ? ` (${fileMb} MB)` : ""} exceeds the ${maxMb} MB preview limit. Please download it to view.`}
                    />
                ) : (
                    <PluginComponent
                        info={fileInfo}
                        registerFeatures={setFeatures}
                    />
                )}
            </div>
        </div>
    );
};

export const FilePreviewCore: React.FC<FilePreviewCoreProps> = (props) => {
    return (
        <FilePreviewMenuProvider>
            <FilePreviewInner {...props} />
        </FilePreviewMenuProvider>
    );
};

export function FilePreviewView() {
    const { id } = useParams<{ id: string }>();
    return <FilePreviewCore fileId={id!} />;
}

export default FilePreviewCore;
