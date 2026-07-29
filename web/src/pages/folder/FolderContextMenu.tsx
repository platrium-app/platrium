import React from "react"
import { FolderPlus, FileUp, FolderUp } from "lucide-react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"

export interface FolderContextMenuProps {
  children: React.ReactNode
}

export function FolderContextMenu({ children }: FolderContextMenuProps) {
  return (
    <ContextMenu>
      <ContextMenuTrigger
        render={
          React.isValidElement(children) ? children : <span>{children}</span>
        }
      />
      <ContextMenuContent className="w-64">
        <ContextMenuGroup>
          <ContextMenuItem className="cursor-pointer">
            <FolderPlus className="mr-2 h-4 w-4" />
            <span>New Folder</span>
          </ContextMenuItem>
        </ContextMenuGroup>
        <ContextMenuSeparator />
        <ContextMenuGroup>
          <ContextMenuItem className="cursor-pointer">
            <FileUp className="mr-2 h-4 w-4" />
            <span>Upload File</span>
          </ContextMenuItem>
          <ContextMenuItem className="cursor-pointer">
            <FolderUp className="mr-2 h-4 w-4" />
            <span>Upload Folder</span>
          </ContextMenuItem>
        </ContextMenuGroup>
      </ContextMenuContent>
    </ContextMenu>
  )
}
