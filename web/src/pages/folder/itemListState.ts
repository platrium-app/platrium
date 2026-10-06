import * as React from "react"
import type { DriveItemNode, SortDirection, SortField } from "./FolderViewTypes"

/** Sorting for a list of drive items: folders always first, then by the chosen column. */
export function useItemSort(rawItems: DriveItemNode[]) {
  const [sortField, setSortField] = React.useState<SortField>("name")
  const [sortDirection, setSortDirection] = React.useState<SortDirection>("asc")

  const items = React.useMemo(() => {
    const sorted = [...rawItems]
    sorted.sort((a, b) => {
      const isAFolder = a.type === "FOLDER"
      const isBFolder = b.type === "FOLDER"
      if (isAFolder && !isBFolder) return -1
      if (!isAFolder && isBFolder) return 1

      let valA: string | number = a[sortField] ?? ""
      let valB: string | number = b[sortField] ?? ""
      if (typeof valA === "string") valA = valA.toLowerCase()
      if (typeof valB === "string") valB = valB.toLowerCase()

      if (valA < valB) return sortDirection === "asc" ? -1 : 1
      if (valA > valB) return sortDirection === "asc" ? 1 : -1
      return 0
    })
    return sorted
  }, [rawItems, sortField, sortDirection])

  const handleSortChange = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((prev) => (prev === "asc" ? "desc" : "asc"))
    } else {
      setSortField(field)
      setSortDirection("asc")
    }
  }

  return { items, sortField, sortDirection, handleSortChange }
}

/** Click, ctrl/cmd-click and shift-click selection over a list of items in display order. */
export function useItemSelection(items: DriveItemNode[]) {
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set())
  const [lastSelectedIndex, setLastSelectedIndex] = React.useState<number | null>(null)

  const clearSelection = React.useCallback(() => {
    setSelectedIds(new Set())
    setLastSelectedIndex(null)
  }, [])

  const handleItemClick = (clicked: DriveItemNode, e: React.MouseEvent) => {
    e.stopPropagation()
    const index = items.findIndex((i) => i.id === clicked.id)

    if (e.metaKey || e.ctrlKey) {
      setSelectedIds((prev) => {
        const next = new Set(prev)
        if (next.has(clicked.id)) next.delete(clicked.id)
        else next.add(clicked.id)
        return next
      })
      setLastSelectedIndex(index)
    } else if (e.shiftKey && lastSelectedIndex !== null && lastSelectedIndex !== index) {
      const start = Math.min(lastSelectedIndex, index)
      const end = Math.max(lastSelectedIndex, index)
      setSelectedIds(new Set(items.slice(start, end + 1).map((i) => i.id)))
    } else {
      setSelectedIds(new Set([clicked.id]))
      setLastSelectedIndex(index)
    }
  }

  const handleItemContextMenu = (clicked: DriveItemNode) => {
    if (!selectedIds.has(clicked.id)) setSelectedIds(new Set([clicked.id]))
  }

  return { selectedIds, setSelectedIds, clearSelection, handleItemClick, handleItemContextMenu }
}
