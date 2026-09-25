import React, { useEffect } from "react"
import { type FilePreviewPluginProps, type PluginDefinition } from "../PluginDefinition"
import { PdfRenderer } from "@/components/pdfrenderer"
import { constructRawContentUrl } from "@/lib/utils"
import { useState } from "react"
import { useRegisterMenu } from "../FilePreviewMenuContext"
import { Maximize, MoveHorizontal, RotateCw, RotateCcw } from "lucide-react"

const PdfPreviewPluginComponent: React.FC<FilePreviewPluginProps> = ({
  info,
  registerFeatures,
  onLoaded,
}) => {
  const [zoomLevel, setZoomLevel] = useState(1.0)
  const [orientation, setOrientation] = useState(0)
  const [fitMode, setFitMode] = useState<"width" | "page" | "auto" | null>("auto")

  useEffect(() => {
    registerFeatures({})
    onLoaded()
  }, [registerFeatures, onLoaded])

  useRegisterMenu("View", [
    {
      id: "pdf-fit-page",
      label: "Fit Page",
      category: "View",
      icon: Maximize,
      onClick: () => setFitMode("page"),
    },
    {
      id: "pdf-fit-width",
      label: "Fit Width",
      category: "View",
      icon: MoveHorizontal,
      onClick: () => setFitMode("width"),
    },
    {
      id: "pdf-rotate-right",
      label: "Rotate Right",
      category: "View",
      icon: RotateCw,
      separatorBefore: true,
      onClick: () => setOrientation((r) => (r + 90) % 360),
    },
    {
      id: "pdf-rotate-left",
      label: "Rotate Left",
      category: "View",
      icon: RotateCcw,
      onClick: () => setOrientation((r) => (r - 90 + 360) % 360),
    },
  ])

  const fileUrl = constructRawContentUrl(info.fileId)

  return (
    <div className="flex-1 w-full h-full bg-background overflow-hidden relative">
      <PdfRenderer 
        fileUrl={fileUrl} 
        zoomLevel={zoomLevel} 
        setZoomLevel={setZoomLevel} 
        orientation={orientation} 
        fitMode={fitMode}
        setFitMode={setFitMode}
      />
    </div>
  )
}

export const PdfPreviewPlugin: PluginDefinition = {
  id: "pdf-previewer",
  name: "PDF Previewer",
  supportedMimeTypes: ["application/pdf"],
  maxSizeBytes: 500 * 1024 * 1024, // 500 MB (because it is completely virtualized and streamed via range requests)
  component: PdfPreviewPluginComponent,
}

export default PdfPreviewPlugin;
