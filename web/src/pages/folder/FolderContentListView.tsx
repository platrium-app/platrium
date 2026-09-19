import * as React from "react"
import { useVirtualizer } from "@tanstack/react-virtual"
import {
    Folder,
    FileText,
    FileImage,
    FileCode,
    FileArchive,
    File,
    ChevronUp,
    ChevronDown,
    MoreVertical,
    Download,
    Share2,
    Pencil,
    Trash2,
    Info,
} from "lucide-react"
import type { FolderViewProps, SortField } from "./FolderViewTypes"
import { ItemContextMenu } from "./ItemContextMenu"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

function formatBytes(bytes?: number | null): string {
    if (bytes == null || bytes === 0) return "--"
    const k = 1024
    const sizes = ["B", "KB", "MB", "GB", "TB"]
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

function formatDate(dateStr?: string): string {
    if (!dateStr) return "--"
    try {
        const d = new Date(dateStr)
        return d.toLocaleDateString("en-US", {
            month: "short",
            day: "numeric",
            year: "numeric",
        })
    } catch {
        return dateStr
    }
}

function getItemIcon(type: string, mimeType?: string | null) {
    if (type === "FOLDER") {
        return <Folder className="size-5 shrink-0 text-blue-500 fill-blue-500/20" />
    }

    if (mimeType?.startsWith("image/")) {
        return <FileImage className="size-5 shrink-0 text-purple-500" />
    }
    if (mimeType?.includes("json") || mimeType?.includes("javascript") || mimeType?.includes("go")) {
        return <FileCode className="size-5 shrink-0 text-amber-500" />
    }
    if (mimeType?.includes("zip") || mimeType?.includes("tar")) {
        return <FileArchive className="size-5 shrink-0 text-amber-600" />
    }
    if (mimeType?.includes("text") || mimeType?.includes("pdf")) {
        return <FileText className="size-5 shrink-0 text-blue-600" />
    }

    return <File className="size-5 shrink-0 text-muted-foreground" />
}

export function FolderContentListView({
    items,
    selectedIds,
    onItemClick,
    onItemDoubleClick,
    onItemContextMenu,
    sortField,
    sortDirection,
    onSortChange,
}: FolderViewProps) {
    const parentRef = React.useRef<HTMLDivElement>(null)

    const rowVirtualizer = useVirtualizer({
        count: items.length,
        getScrollElement: () => parentRef.current,
        estimateSize: () => 48,
        overscan: 10,
    })

    const renderSortArrow = (field: SortField) => {
        if (sortField !== field) return null
        return sortDirection === "asc" ? (
            <ChevronUp className="size-3.5 inline ml-1" />
        ) : (
            <ChevronDown className="size-3.5 inline ml-1" />
        )
    }

    return (
        <div className="flex max-h-full w-full flex-col overflow-hidden shrink-0">
            {/* Table Header */}
            <div className="grid grid-cols-12 gap-2 border-b px-4 py-3 text-sm font-medium text-muted-foreground select-none">
                <div
                    className="col-span-5 flex cursor-pointer items-center transition-colors hover:text-foreground"
                    onClick={() => onSortChange("name")}
                >
                    <span>Name</span>
                    {renderSortArrow("name")}
                </div>
                <div
                    className="col-span-2 hidden cursor-pointer items-center transition-colors hover:text-foreground sm:flex"
                    onClick={() => onSortChange("owner")}
                >
                    <span>Owner</span>
                    {renderSortArrow("owner")}
                </div>
                <div
                    className="col-span-2 hidden cursor-pointer items-center transition-colors hover:text-foreground md:flex"
                    onClick={() => onSortChange("updatedAt")}
                >
                    <span>Last Modified</span>
                    {renderSortArrow("updatedAt")}
                </div>
                <div
                    className="col-span-5 sm:col-span-2 md:col-span-2 flex cursor-pointer items-center justify-end text-right transition-colors hover:text-foreground"
                    onClick={() => onSortChange("size")}
                >
                    <span>File Size</span>
                    {renderSortArrow("size")}
                </div>
                <div className="col-span-2 sm:col-span-1 flex items-center justify-end" />
            </div>

            {/* Virtualized Rows Container */}
            <div ref={parentRef} className="flex-1 overflow-auto">
                <div
                    style={{
                        height: `${rowVirtualizer.getTotalSize()}px`,
                        width: "100%",
                        position: "relative",
                    }}
                >
                    {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                        const item = items[virtualRow.index]
                        const isSelected = selectedIds.has(item.id)

                        return (
                            <ItemContextMenu key={item.id} item={item}>
                                <div
                                    data-index={virtualRow.index}
                                    ref={rowVirtualizer.measureElement}
                                    style={{
                                        position: "absolute",
                                        top: 0,
                                        left: 0,
                                        width: "100%",
                                        transform: `translateY(${virtualRow.start}px)`,
                                    }}
                                    className={cn(
                                        "group grid grid-cols-12 items-center gap-2 border-b px-4 py-3 text-sm transition-colors cursor-pointer select-none",
                                        isSelected
                                            ? "bg-primary/10 text-foreground"
                                            : "bg-transparent hover:bg-muted/50 text-foreground/90"
                                    )}
                                    onClick={(e) => onItemClick(item, e)}
                                    onDoubleClick={() => onItemDoubleClick(item)}
                                    onContextMenu={(e) => onItemContextMenu(item, e)}
                                >
                                    {/* Name Column */}
                                    <div className="col-span-5 flex items-center gap-2.5 truncate">
                                        {getItemIcon(item.type, item.mimeType)}
                                        <span className="truncate">{item.name}</span>
                                    </div>

                                    {/* Owner Column */}
                                    <div className="col-span-2 hidden truncate text-xs text-muted-foreground sm:block">
                                        {item.owner || "me"}
                                    </div>

                                    {/* Last Modified Column */}
                                    <div className="col-span-2 hidden truncate text-xs text-muted-foreground md:block">
                                        {formatDate(item.updatedAt)}
                                    </div>

                                    {/* Size Column */}
                                    <div className="col-span-5 sm:col-span-2 md:col-span-2 text-right text-xs text-muted-foreground font-mono">
                                        {formatBytes(item.size)}
                                    </div>

                                    {/* Item Options Dropdown Button */}
                                    <div className="col-span-2 sm:col-span-1 flex items-center justify-end">
                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            className="size-7 opacity-0 group-hover:opacity-100 transition-opacity"
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
                                            <MoreVertical className="size-4 text-muted-foreground" />
                                        </Button>
                                    </div>
                                </div>
                            </ItemContextMenu>
                        )
                    })}
                </div>
            </div>
        </div>
    )
}

