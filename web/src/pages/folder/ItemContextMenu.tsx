import React from "react"
import { Download, Share2, Pencil, Trash2, Info, ArrowUpDown, FileUp } from "lucide-react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import type { DriveItemNode, DriveOperation } from "./FolderViewTypes"
import { allCan } from "@/lib/capabilities"
import { triggerFileDownload } from "@/lib/utils"

export interface ItemContextMenuProps {
  children: React.ReactNode
  item: DriveItemNode
  selectedItems?: DriveItemNode[]
  onOperation?: (mode: DriveOperation, items: DriveItemNode[]) => void
}

export function ItemContextMenu({ children, item, selectedItems = [item], onOperation }: ItemContextMenuProps) {
  const isFolder = item.type === "FOLDER"
  const isMulti = selectedItems.length > 1
  const allFiles = selectedItems.every(i => i.type === "FILE")
  // What the server says this user may do. Hidden actions would be refused anyway.
  const canDownload = allCan(selectedItems, "DOWNLOAD")
  const canShare = !isMulti && allCan(selectedItems, "SHARE")
  const canRename = !isMulti && allCan(selectedItems, "EDIT")
  const canMove = allCan(selectedItems, "MOVE")
  const canCopy = !isMulti && allFiles && canDownload

  return (
    <ContextMenu>
      <ContextMenuTrigger render={children as React.ReactElement} />
      <ContextMenuContent className="w-48">
        {allFiles && canDownload && (
          <ContextMenuItem onClick={() => selectedItems.forEach(i => triggerFileDownload(i.id, i.name))}>
            <Download className="size-4 text-muted-foreground" />
            <span>Download</span>
          </ContextMenuItem>
        )}
        {canShare && (
          <ContextMenuItem onClick={() => onOperation?.("SHARE", selectedItems)}>
            <Share2 className="size-4 text-muted-foreground" />
            <span>Share</span>
          </ContextMenuItem>
        )}

        {canRename && (
          <ContextMenuItem onClick={() => onOperation?.("RENAME", selectedItems)}>
            <Pencil className="size-4 text-muted-foreground" />
            <span>Rename</span>
          </ContextMenuItem>
        )}

        {canMove && (
          <ContextMenuItem onClick={() => onOperation?.("MOVE", selectedItems)}>
            <ArrowUpDown className="size-4 text-muted-foreground" />
            <span>Move</span>
          </ContextMenuItem>
        )}

        {canCopy && (
          <ContextMenuItem onClick={() => onOperation?.("COPY", selectedItems)}>
            <FileUp className="size-4 text-muted-foreground" />
            <span>Make a copy</span>
          </ContextMenuItem>
        )}

        <ContextMenuSeparator />
        {!isMulti && (
          <ContextMenuItem>
            <Info className="size-4 text-muted-foreground" />
            <span>{isFolder ? "Folder Information" : "Information"}</span>
          </ContextMenuItem>
        )}
        <ContextMenuItem disabled variant="destructive">
          <Trash2 className="size-4" />
          <span>Delete</span>
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
