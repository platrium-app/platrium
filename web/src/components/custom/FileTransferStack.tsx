import { useState } from "react"
import { useUpload } from "@/contexts/UploadContext"
import { FileTransferCard } from "./FileTransferCard"
import { Spinner } from "@/components/ui/spinner"
import {
  ChevronDown,
  CheckCircle2,
  XCircle,
  Trash2,
} from "lucide-react"

export function FileTransferStack() {
  const { transfers, cancelTransfer, clearCompleted } = useUpload()
  const [isExpanded, setIsExpanded] = useState(true)

  if (transfers.length === 0) {
    return null
  }

  const activeTransfers = transfers.filter(
    (t) => t.status.type === "Preparing" || t.status.type === "Transferring"
  )
  const completedCount = transfers.filter((t) => t.status.type === "Completed").length
  const cancelledCount = transfers.filter((t) => t.status.type === "Cancelled").length
  const errorCount = transfers.filter((t) => t.status.type === "Error").length

  const isUploading = activeTransfers.length > 0
  const hasFailures = cancelledCount > 0 || errorCount > 0

  const headerTitle = isUploading
    ? `Transferring ${activeTransfers.length} file${activeTransfers.length > 1 ? "s" : ""}`
    : completedCount === transfers.length
    ? `All Transfers Completed (${completedCount})`
    : completedCount > 0
    ? `Transfers (${completedCount}/${transfers.length} Completed)`
    : cancelledCount === transfers.length
    ? `Transfers Cancelled (${cancelledCount})`
    : errorCount > 0
    ? `Transfers Failed (${errorCount})`
    : `Transfers (${transfers.length})`

  return (
    <div className="fixed z-50 select-none bg-background/95 p-0 shadow-2xl backdrop-blur-xl transition-all duration-300 dark:bg-zinc-900/95 bottom-0 inset-x-0 w-full rounded-none border-t border-border/60 dark:border-white/10 sm:bottom-5 sm:right-5 sm:left-auto sm:w-96 sm:rounded-2xl sm:border">
      {/* Header */}
      <div
        onClick={() => setIsExpanded(!isExpanded)}
        className="flex items-center justify-between px-4 py-3 cursor-pointer select-none"
      >
        <div className="flex items-center gap-2.5 overflow-hidden">
          {isUploading ? (
            <Spinner className="h-4 w-4 text-primary shrink-0" />
          ) : hasFailures ? (
            <XCircle className="h-4 w-4 text-destructive shrink-0" />
          ) : (
            <CheckCircle2 className="h-4 w-4 text-emerald-500 shrink-0" />
          )}
          <span className="truncate text-xs font-semibold tracking-tight text-foreground">
            {headerTitle}
          </span>
        </div>

        <div className="flex items-center gap-1 shrink-0" onClick={(e) => e.stopPropagation()}>
          {!isUploading && transfers.length > 0 && (
            <button
              onClick={clearCompleted}
              className="rounded-lg p-1 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
              title="Clear finished transfers"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          )}
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="rounded-lg p-1 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
            title={isExpanded ? "Collapse" : "Expand"}
          >
            <ChevronDown
              className={`h-4 w-4 transition-transform duration-300 ease-in-out ${
                isExpanded ? "" : "rotate-180"
              }`}
            />

          </button>
        </div>
      </div>

      {/* Smooth Animated Expansion & Contraction List Container */}
      <div
        className={`grid transition-all duration-300 ease-in-out ${
          isExpanded ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"
        }`}
      >
        <div className="overflow-hidden">
          <div className="max-h-[30vh] overflow-y-auto p-1.5 flex flex-col gap-1">
            {transfers.map((item) => (
              <FileTransferCard key={item.transferId} item={item} onCancel={cancelTransfer} />
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
