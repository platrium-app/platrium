import React, { useState, useEffect, useCallback } from "react";
import { useParams } from "react-router-dom";
import { previewRegistry } from "./PluginRegistry";
import { DEFAULT_MAX_PREVIEW_SIZE_BYTES, type FilePreviewInfo, type PreviewFeatures } from "./PluginDefinition";
import { FallbackPlugin } from "./plugins/FallbackPlugin";
import { FilePreviewMenuBar } from "./FilePreviewMenuBar";
import { Spinner } from "@/components/ui/spinner";
import { PlaceholderView } from "@/components/custom/PlaceholderView";
import { CloudAlertIcon } from "lucide-react";
import { constructRawContentUrl, parseFilenameFromContentDisposition } from "@/lib/utils";

import { useServiceWorkerReady } from "@/hooks/useServiceWorkerReady";

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
    const { isControlling } = useServiceWorkerReady(true);
    const [metadata, setMetadata] = useState<FileMetadata | null>(null);
    const [error, setError] = useState<Error | null>(null);
    const [features, setFeatures] = useState<PreviewFeatures>({});
    const [isContentLoaded, setIsContentLoaded] = useState(false);
    const handleLoaded = useCallback(() => setIsContentLoaded(true), []);

    useEffect(() => {
        setIsContentLoaded(false);
        setMetadata(null);
        setError(null);
    }, [fileId]);

    useEffect(() => {
        if (!isControlling) return;

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
    }, [fileId, isControlling]);

    const containerClasses = isModal
        ? "flex flex-col h-screen w-screen bg-transparent text-foreground overflow-hidden relative"
        : "flex flex-col h-screen w-screen bg-background text-foreground overflow-hidden relative";

    const pluginDefinition = metadata ? previewRegistry.getPluginForMimeType(metadata.mimeType) : null;
    const effectiveMaxSize = pluginDefinition?.maxSizeBytes !== undefined
        ? pluginDefinition.maxSizeBytes
        : DEFAULT_MAX_PREVIEW_SIZE_BYTES;
    const isTooLarge = metadata && effectiveMaxSize > 0 && metadata.sizeBytes !== undefined && metadata.sizeBytes > effectiveMaxSize;

    const fileInfo: FilePreviewInfo | null = metadata ? {
        fileId,
        fileName: metadata.name,
        mimeType: metadata.mimeType,
        sizeBytes: metadata.sizeBytes,
    } : null;

    const PluginComponent = pluginDefinition?.component;
    const FallbackComponent = FallbackPlugin.component;

    const maxMb = (effectiveMaxSize / (1024 * 1024)).toFixed(0);
    const fileMb = metadata?.sizeBytes ? (metadata.sizeBytes / (1024 * 1024)).toFixed(1) : undefined;

    const showPluginContent = isControlling && metadata && (isTooLarge || isContentLoaded);

    return (
        <div className={containerClasses}>
            {/* Top Toolbar using FilePreviewMenuBar - rendered from frame 1 */}
            <FilePreviewMenuBar
                info={fileInfo}
                features={features}
                isModal={isModal}
                onClose={onClose}
            />

            {/* Content Area - fixed height (flex-1) from frame 1 */}
            <div className="flex-1 flex flex-col min-h-0 w-full overflow-hidden bg-background/35 dark:bg-transparent relative items-center justify-center">
                {error ? (
                    <PlaceholderView
                        icon={CloudAlertIcon}
                        title="Failed to Fetch File"
                        description={`The following Error Occured: ${error.message}`}
                        variant="error"
                    />
                ) : !showPluginContent ? (
                    <>
                        <PlaceholderView
                            icon={Spinner}
                            title="Hang tight!"
                            description="We're fetching the information about the file."
                            variant="ghost"
                        />
                        {fileInfo && PluginComponent && !isTooLarge && (
                            <div className="hidden">
                                <PluginComponent
                                    info={fileInfo}
                                    registerFeatures={setFeatures}
                                    onLoaded={handleLoaded}
                                    onError={setError}
                                />
                            </div>
                        )}
                    </>
                ) : isTooLarge ? (
                    <FallbackComponent
                        info={fileInfo!}
                        registerFeatures={setFeatures}
                        title="File Too Large to Preview"
                        description={`This file${fileMb ? ` (${fileMb} MB)` : ""} exceeds the ${maxMb} MB preview limit. Please download it to view.`}
                        onLoaded={handleLoaded}
                    />
                ) : (
                    PluginComponent && (
                        <PluginComponent
                            info={fileInfo!}
                            registerFeatures={setFeatures}
                            onLoaded={handleLoaded}
                            onError={setError}
                        />
                    )
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
