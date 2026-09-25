import React, { useEffect } from "react";
import type { FilePreviewPluginProps, PluginDefinition } from "../PluginDefinition";
import { Button } from "@/components/ui/button";
import { PlaceholderView } from "@/components/custom/PlaceholderView";
import { triggerFileDownload } from "@/lib/utils";
import { Download, AlertTriangle } from "lucide-react";

const EMPTY_FEATURES = {};

const FallbackPluginComponent: React.FC<FilePreviewPluginProps> = ({
  info,
  registerFeatures,
  title,
  description,
  onLoaded,
}) => {
  useEffect(() => {
    // Fallback plugin doesn't have special features like zoom or pages.
    registerFeatures(EMPTY_FEATURES);
    onLoaded();
  }, [registerFeatures, onLoaded]);

  const handleDownload = () => {
    triggerFileDownload(info.fileId, info.fileName);
  };

  const displayTitle = title || "No Preview Available";
  const displayDescription =
    description ||
    `We don't support previewing this file type. Please download it to view the contents.`;

  return (
    <PlaceholderView
      icon={AlertTriangle}
      title={displayTitle}
      description={displayDescription}
      variant="warning"
      className="h-full w-full"
      action={
        <Button onClick={handleDownload} size="lg">
          <Download className="mr-2 h-4 w-4" />
          Download File
        </Button>
      }
    />
  );
};

export const FallbackPlugin: PluginDefinition = {
  id: "fallback",
  name: "Fallback Previewer",
  supportedMimeTypes: [], // Fallback matches nothing natively, it's used when no match is found
  component: FallbackPluginComponent,
};
