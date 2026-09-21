import type React from "react"

export type DriveItemTypeEnum = "FILE" | "FOLDER"

export interface DriveItemNode {
  id: string
  parentId?: string | null
  name: string
  type: DriveItemTypeEnum
  size?: number | null
  mimeType?: string | null
  createdAt: string
  updatedAt: string
  owner?: string
}

export type SortField = "name" | "updatedAt" | "size" | "type" | "owner"
export type SortDirection = "asc" | "desc"
export type ViewMode = "list" | "grid"

export interface FolderViewProps {
  items: DriveItemNode[]
  selectedIds: Set<string>
  onSelectionChange: (selectedIds: Set<string>) => void
  onItemClick: (item: DriveItemNode, e: React.MouseEvent) => void
  onItemDoubleClick: (item: DriveItemNode) => void
  onItemContextMenu: (item: DriveItemNode, e: React.MouseEvent) => void
  sortField: SortField
  sortDirection: SortDirection
  onSortChange: (field: SortField) => void
}
