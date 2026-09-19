import * as React from "react"
import {
  Folder,
  FileText,
  FileImage,
  FileCode,
  FileArchive,
  File,
  MoreVertical,
  Download,
  Share2,
  Pencil,
  Trash2,
  Info,
} from "lucide-react"
import type { FolderViewProps } from "./FolderViewTypes"
import { ItemContextMenu } from "./ItemContextMenu"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

function getItemIcon(type: string, mimeType?: string | null) {
  if (type === "FOLDER") {
    return <Folder className="size-10 text-blue-500 fill-blue-500/20" />
  }

  if (mimeType?.startsWith("image/")) {
    return <FileImage className="size-10 text-purple-500" />
  }
  if (mimeType?.includes("json") || mimeType?.includes("javascript") || mimeType?.includes("go")) {
    return <FileCode className="size-10 text-amber-500" />
  }
  if (mimeType?.includes("zip") || mimeType?.includes("tar")) {
    return <FileArchive className="size-10 text-amber-600" />
  }
  if (mimeType?.includes("text") || mimeType?.includes("pdf")) {
    return <FileText className="size-10 text-blue-600" />
  }

  return <File className="size-10 text-muted-foreground" />
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return ""
  try {
    const d = new Date(dateStr)
    return d.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
    })
  } catch {
    return dateStr
  }
}

export function FolderContentGridView({
  items,
  selectedIds,
  onItemClick,
  onItemDoubleClick,
  onItemContextMenu,
}: FolderViewProps) {
  return (
    <div className="w-full max-h-full overflow-auto py-2">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
        {items.map((item) => {
          const isSelected = selectedIds.has(item.id)

          return (
            <ItemContextMenu key={item.id} item={item}>
              <div
                className={cn(
                  "group relative flex flex-col items-center justify-between rounded-xl border p-3 text-center transition-all cursor-pointer select-none",
                  isSelected
                    ? "border-primary bg-primary/10 ring-2 ring-primary/40 shadow-xs"
                    : "border-border/60 bg-card hover:border-primary/50 hover:bg-accent/40"
                )}
                onClick={(e) => onItemClick(item, e)}
                onDoubleClick={() => onItemDoubleClick(item)}
                onContextMenu={(e) => onItemContextMenu(item, e)}
              >
                {/* Item Options Dropdown Button */}
                <div className="absolute right-1 top-1 opacity-0 group-hover:opacity-100 transition-opacity z-10">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-6 p-0"
                    onClick={(e) => {
                      e.stopPropagation()
                      e.currentTarget.dispatchEvent(
                        new MouseEvent("contextmenu", {
                          bubbles: true,
                          clientX: e.clientX,
                          clientY: e.clientY,
                        })
                      )
                    }}
                  >
                    <MoreVertical className="size-3.5 text-muted-foreground" />
                  </Button>
                </div>

                <div className="mb-2 mt-1 flex size-14 items-center justify-center rounded-lg bg-muted/30 group-hover:bg-muted/60 transition-colors">
                  {getItemIcon(item.type, item.mimeType)}
                </div>

                <div className="w-full">
                  <p className="truncate text-xs font-medium text-foreground">
                    {item.name}
                  </p>
                  <p className="mt-0.5 text-[10px] text-muted-foreground">
                    {formatDate(item.updatedAt)}
                  </p>
                </div>
              </div>
            </ItemContextMenu>
          )
        })}
      </div>
    </div>
  )
}
