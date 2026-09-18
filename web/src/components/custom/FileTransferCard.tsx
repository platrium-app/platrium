import type { NetTransferEvent } from "platrium-sdk"
import { CircularProgress } from "./CircularProgress"
import {
  FileIcon,
  FileText,
  FileImage,
  FileCode,
  FileArchive,
  CheckCircle2,
  AlertCircle,
  Upload,
  Download,
  X,
} from "lucide-react"

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B"
  const k = 1024
  const sizes = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i]
}

function getFileIcon(fileName: string) {
  const ext = fileName.split(".").pop()?.toLowerCase() || ""
  if (["png", "jpg", "jpeg", "gif", "webp", "svg"].includes(ext)) {
    return <FileImage className="h-4 w-4 text-blue-400 shrink-0" />
  }
  if (["js", "ts", "tsx", "jsx", "rs", "go", "html", "css", "json"].includes(ext)) {
    return <FileCode className="h-4 w-4 text-amber-400 shrink-0" />
  }
  if (["zip", "tar", "gz", "rar", "7z"].includes(ext)) {
    return <FileArchive className="h-4 w-4 text-purple-400 shrink-0" />
  }
  if (["pdf", "doc", "docx", "txt", "md"].includes(ext)) {
    return <FileText className="h-4 w-4 text-emerald-400 shrink-0" />
  }
  return <FileIcon className="h-4 w-4 text-muted-foreground shrink-0" />
}

export function FileTransferCard({
  item,
  onCancel,
}: {
  item: NetTransferEvent
  onCancel: (transferId: string) => void
}) {
  const fileName =
    item.metadata && item.metadata.type === "FileChunk"
      ? item.metadata.file_name
      : "File"
  const statusType = item.status.type
  const errorMessage = item.status.type === "Error" ? item.status.message : ""

  const percent =
    item.totalBytes > 0
      ? Math.min(100, Math.round((item.bytesTransferred / item.totalBytes) * 100))
      : 0

  const isUploading = statusType === "Transferring" || statusType === "Preparing"

  return (
    <div className="group relative flex items-center gap-3 rounded-xl p-2.5 h-[52px] transition-colors hover:bg-accent/40">
      {/* Left: Vertically Centered File Icon */}
      <div className="shrink-0">{getFileIcon(fileName)}</div>

      {/* Middle Column: Fixed 2 Lines */}
      <div className="flex flex-col flex-1 min-w-0 justify-center gap-0.5 overflow-hidden">
        {/* Line 1: Filename */}
        <p
          className={`truncate text-xs font-medium ${isUploading ? "shimmer text-muted-foreground" : "text-foreground"
            }`}
        >
          {fileName}
        </p>

        {/* Line 2: Metrics / Status Line */}
        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground min-w-0 truncate">
          <span className="flex items-center gap-0.5 shrink-0">
            {item.direction === "Upload" ? (
              <Upload className="h-2.5 w-2.5 text-blue-400" />
            ) : (
              <Download className="h-2.5 w-2.5 text-emerald-400" />
            )}
            {item.direction}
          </span>
          <span className="shrink-0">•</span>

          {statusType === "Transferring" && (
            <span className="truncate">
              {percent}% ({formatBytes(item.bytesTransferred)} / {formatBytes(item.totalBytes)})
            </span>
          )}
          {statusType === "Preparing" && <span className="truncate">Preparing transfer...</span>}
          {statusType === "Completed" && <span className="truncate">{formatBytes(item.totalBytes)}</span>}
          {statusType === "Cancelled" && <span className="truncate">Cancelled</span>}
          {statusType === "Error" && (
            <span className="text-destructive font-medium truncate" title={errorMessage}>
              Upload failed
            </span>
          )}
        </div>
      </div>

      {/* Right: Balanced 18px Status Icons inside 24px Container */}
      <div className="flex items-center justify-center shrink-0 w-7 h-7">
        {isUploading && (
          <CircularProgress
            value={percent}
            size={24}
            innerIcon={X}
            onClick={() => onCancel(item.transferId)}
            title="Cancel Transfer"
          />
        )}
        {statusType === "Completed" && (
          <CheckCircle2 className="h-5 w-5 text-emerald-500 shrink-0" />
        )}
        {statusType === "Error" && (
          <span title={errorMessage || "Upload failed"} className="flex items-center justify-center h-[18px] w-[18px]">
            <AlertCircle className="h-5 w-5 text-destructive shrink-0" />
          </span>
        )}
      </div>
    </div>
  )
}
