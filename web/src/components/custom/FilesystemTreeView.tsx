import React, { useState } from "react"
import { ChevronDown, ChevronRight, Folder } from "lucide-react"
import { SidebarMenuItem, SidebarMenuButton } from "@/components/ui/sidebar"

export interface FileTreeNode {
  id: string
  name: string
  hasChildren: boolean
  icon?: React.ElementType
}

export interface FilesystemTreeProps {
  nodes: FileTreeNode[]
  getChildren: (nodeId: string) => FileTreeNode[]
  activeId?: string
  onSelect?: (nodeId: string) => void
  level?: number
}

export function FilesystemTree({
  nodes,
  getChildren,
  activeId,
  onSelect,
  level = 0,
}: FilesystemTreeProps) {
  return (
    <>
      {nodes.map((node) => (
        <FilesystemTreeItem
          key={node.id}
          node={node}
          getChildren={getChildren}
          activeId={activeId}
          onSelect={onSelect}
          level={level}
        />
      ))}
    </>
  )
}

function FilesystemTreeItem({
  node,
  getChildren,
  activeId,
  onSelect,
  level,
}: {
  node: FileTreeNode
  getChildren: (nodeId: string) => FileTreeNode[]
  activeId?: string
  onSelect?: (nodeId: string) => void
  level: number
}) {
  const [isExpanded, setIsExpanded] = useState(false)
  const isActive = activeId === node.id
  const NodeIcon = node.icon || Folder
  const childrenNodes = isExpanded ? getChildren(node.id) : []

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        isActive={isActive}
        onClick={() => onSelect?.(node.id)}
        style={{ paddingLeft: `${level * 12 + 8}px` }}
      >
        {node.hasChildren ? (
          isExpanded ? (
            <ChevronDown
              className="size-4 flex-shrink-0 cursor-pointer"
              onClick={(e) => {
                e.stopPropagation()
                setIsExpanded(false)
              }}
            />
          ) : (
            <ChevronRight
              className="size-4 flex-shrink-0 cursor-pointer"
              onClick={(e) => {
                e.stopPropagation()
                setIsExpanded(true)
              }}
            />
          )
        ) : (
          <div className="size-4 flex-shrink-0" />
        )}

        <NodeIcon className="size-4 flex-shrink-0" />
        <span className="truncate">{node.name}</span>
      </SidebarMenuButton>

      {node.hasChildren && isExpanded && childrenNodes.length > 0 && (
        <ul className="relative flex w-full min-w-0 flex-col gap-0.5">
          <div
            className="pointer-events-none absolute top-0 bottom-0 z-10 w-px bg-sidebar-border opacity-0 transition-opacity duration-200 group-hover/tree:opacity-100"
            style={{ left: `${level * 12 + 16}px` }}
          />
          <FilesystemTree
            nodes={childrenNodes}
            getChildren={getChildren}
            activeId={activeId}
            onSelect={onSelect}
            level={level + 1}
          />
        </ul>
      )}
    </SidebarMenuItem>
  )
}
