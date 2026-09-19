import React from "react"
import { Download, Share2, Pencil, Trash2, Info } from "lucide-react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import type { DriveItemNode } from "./FolderViewTypes"

export interface ItemContextMenuProps {
  children: React.ReactNode
  item: DriveItemNode
}

export function ItemContextMenu({ children, item }: ItemContextMenuProps) {
  const isFolder = item.type === "FOLDER"

  return (
    <ContextMenu>
      <ContextMenuTrigger render={children as React.ReactElement} />
      <ContextMenuContent className="w-48">
        <ContextMenuItem disabled>
          <Download className="size-4 text-muted-foreground" />
          <span>Download</span>
        </ContextMenuItem>
        <ContextMenuItem disabled>
          <Share2 className="size-4 text-muted-foreground" />
          <span>Share</span>
        </ContextMenuItem>
        <ContextMenuItem disabled>
          <Pencil className="size-4 text-muted-foreground" />
          <span>Rename</span>
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem>
          <Info className="size-4 text-muted-foreground" />
          <span>{isFolder ? "Folder Information" : "File Information"}</span>
        </ContextMenuItem>
        <ContextMenuItem disabled variant="destructive">
          <Trash2 className="size-4" />
          <span>Delete</span>
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
