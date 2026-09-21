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
import { useUpload } from "@/contexts/UploadContext"

export interface FolderContextMenuProps {
    children: React.ReactNode
    folderId: string
}

export function FolderContextMenu({ children, folderId }: FolderContextMenuProps) {
    const { triggerUpload } = useUpload()

    return (
        <ContextMenu>
            <ContextMenuTrigger className="flex min-h-full w-full flex-1 flex-col outline-none">
                {children}
            </ContextMenuTrigger>
            <ContextMenuContent className="w-64">
                <ContextMenuGroup>
                    <ContextMenuItem className="cursor-pointer">
                        <FolderPlus className="mr-2 h-4 w-4" />
                        <span>New Folder</span>
                    </ContextMenuItem>
                </ContextMenuGroup>
                <ContextMenuSeparator />
                <ContextMenuGroup>
                    <ContextMenuItem
                        className="cursor-pointer"
                        onClick={() => triggerUpload(folderId)}
                    >
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

