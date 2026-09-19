import * as React from "react"
import { useParams, useNavigate } from "react-router-dom"
import { FolderOpen, AlertTriangle, FolderRoot, Folder } from "lucide-react"
import { FolderContextMenu } from "./FolderContextMenu"
import { useSetBreadcrumbs, type BreadcrumbItemType } from "@/contexts/BreadcrumbContext"
import { useQuery } from "@apollo/client/react"
import { graphql } from "@/graphql"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { Spinner } from "@/components/ui/spinner"
import { FolderHeaderToolbar } from "./FolderHeaderToolbar"
import { FolderContentListView } from "./FolderContentListView"
import { FolderContentGridView } from "./FolderContentGridView"
import { SelectionArea } from "@/components/custom/SelectionArea"
import type { DriveItemNode, SortField, SortDirection, ViewMode } from "./FolderViewTypes"

const GET_FOLDER_INFO = graphql(`
  query GetFolderInfo($id: ID!) {
    item(id: $id) {
      id
      name
      path {
        id
        name
      }
    }
  }
`)

const GET_FOLDER_CONTENTS = graphql(`
  query GetFolderContents($folderId: ID!, $first: Int, $after: String) {
    folderContents(folderId: $folderId, first: $first, after: $after) {
      totalCount
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        cursor
        node {
          id
          parentId
          name
          type
          createdAt
          updatedAt
          ... on File {
            size
            mimeType
          }
        }
      }
    }
  }
`)

export default function FolderRootView() {
  const { id } = useParams()
  const navigate = useNavigate()

  const [viewMode, setViewMode] = React.useState<ViewMode>("list")
  const [sortField, setSortField] = React.useState<SortField>("name")
  const [sortDirection, setSortDirection] = React.useState<SortDirection>("asc")
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set())
  const [lastSelectedIndex, setLastSelectedIndex] = React.useState<number | null>(null)

  // Clear selection on folder navigation
  React.useEffect(() => {
    setSelectedIds(new Set())
    setLastSelectedIndex(null)
  }, [id])

  const infoQuery = useQuery(GET_FOLDER_INFO, {
    variables: { id: id! },
    skip: !id,
  })

  const contentsQuery = useQuery(GET_FOLDER_CONTENTS, {
    variables: { folderId: id!, first: 100 },
    skip: !id,
  })

  const item = infoQuery.data?.item

  // Map GraphQL nodes into DriveItemNode[]
  const rawItems = React.useMemo<DriveItemNode[]>(() => {
    const edges = contentsQuery.data?.folderContents?.edges || []
    return edges.map((edge) => {
      const node = edge.node as any
      return {
        id: node.id,
        parentId: node.parentId ?? null,
        name: node.name,
        type: node.type === "FILE" ? "FILE" : "FOLDER",
        size: node.size ?? null,
        mimeType: node.mimeType ?? null,
        createdAt: node.createdAt,
        updatedAt: node.updatedAt,
        owner: "me",
      }
    })
  }, [contentsQuery.data])

  // Sorted items
  const items = React.useMemo(() => {
    const sorted = [...rawItems]
    sorted.sort((a, b) => {
      // Folders always sorted first
      const isAFolder = a.type === "FOLDER"
      const isBFolder = b.type === "FOLDER"
      if (isAFolder && !isBFolder) return -1
      if (!isAFolder && isBFolder) return 1

      let valA: any = a[sortField] ?? ""
      let valB: any = b[sortField] ?? ""

      if (typeof valA === "string") valA = valA.toLowerCase()
      if (typeof valB === "string") valB = valB.toLowerCase()

      if (valA < valB) return sortDirection === "asc" ? -1 : 1
      if (valA > valB) return sortDirection === "asc" ? 1 : -1
      return 0
    })
    return sorted
  }, [rawItems, sortField, sortDirection])

  // Breadcrumb updates
  const breadcrumbs = React.useMemo<BreadcrumbItemType[]>(() => {
    const crumbs: BreadcrumbItemType[] = []

    if (infoQuery.loading) {
      crumbs.push({
        id: id,
        label: "Loading...",
        href: `/folder/${id}`,
        icon: Spinner,
      })
      return crumbs
    }

    if (item) {
      if (item.path && item.path.length > 0) {
        item.path.forEach((folder, index) => {
          crumbs.push({
            id: folder.id,
            label: folder.name,
            href: `/folder/${folder.id}`,
            icon: index === 0 ? FolderRoot : Folder,
          })
        })
        crumbs.push({
          id: id,
          label: item.name,
          href: `/folder/${id}`,
          icon: Folder,
        })
      } else {
        crumbs.push({
          id: id,
          label: item.name,
          href: `/folder/${id}`,
          icon: FolderRoot,
        })
      }
    } else {
      crumbs.push({
        id: id,
        label: "Invalid Resource",
        href: `/folder/${id}`,
        icon: AlertTriangle,
      })
    }

    return crumbs
  }, [id, item, infoQuery.loading])

  useSetBreadcrumbs(breadcrumbs)

  // Selection Handlers
  const handleItemClick = (clickedItem: DriveItemNode, e: React.MouseEvent) => {
    e.stopPropagation()
    const index = items.findIndex((i) => i.id === clickedItem.id)

    if (e.metaKey || e.ctrlKey) {
      setSelectedIds((prev) => {
        const next = new Set(prev)
        if (next.has(clickedItem.id)) {
          next.delete(clickedItem.id)
        } else {
          next.add(clickedItem.id)
        }
        return next
      })
      setLastSelectedIndex(index)
    } else if (e.shiftKey && lastSelectedIndex !== null && lastSelectedIndex !== index) {
      const start = Math.min(lastSelectedIndex, index)
      const end = Math.max(lastSelectedIndex, index)
      const rangeIds = items.slice(start, end + 1).map((i) => i.id)
      setSelectedIds(new Set(rangeIds))
    } else {
      setSelectedIds(new Set([clickedItem.id]))
      setLastSelectedIndex(index)
    }
  }

  const handleItemDoubleClick = (clickedItem: DriveItemNode) => {
    if (clickedItem.type === "FOLDER") {
      navigate(`/folder/${clickedItem.id}`)
    }
  }

  const handleItemContextMenu = (clickedItem: DriveItemNode) => {
    if (!selectedIds.has(clickedItem.id)) {
      setSelectedIds(new Set([clickedItem.id]))
    }
  }

  const handleSortChange = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((prev) => (prev === "asc" ? "desc" : "asc"))
    } else {
      setSortField(field)
      setSortDirection("asc")
    }
  }

  const isLoading = infoQuery.loading || contentsQuery.loading
  const isError = infoQuery.error || contentsQuery.error

  if (isLoading) {
    return (
      <div className="flex h-full min-h-[50vh] w-full flex-col items-center justify-center p-8 text-center animate-in fade-in duration-300">
        <Spinner className="size-8 text-muted-foreground" />
        <p className="mt-3 text-sm text-muted-foreground">Loading this Resource</p>
      </div>
    )
  }

  if (isError) {
    const errorMsg = infoQuery.error?.message || contentsQuery.error?.message || "Failed to load folder information."
    return (
      <PlaceholderView
        icon={AlertTriangle}
        variant="error"
        title="Error Accessing Resource"
        description={errorMsg}
      />
    )
  }

  if (!item) {
    return (
      <PlaceholderView
        icon={AlertTriangle}
        variant="error"
        title="Resource Not Found"
        description="The requested folder could not be found."
      />
    )
  }

  return (
    <FolderContextMenu folderId={id!}>
      <div className="flex h-full w-full flex-1 flex-col overflow-hidden">
        {/* Permanent Toolbar */}
        <FolderHeaderToolbar
          selectedCount={selectedIds.size}
          onClearSelection={() => setSelectedIds(new Set())}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          sortField={sortField}
          sortDirection={sortDirection}
          onSortChange={handleSortChange}
          folderId={id!}
        />

        {/* Content View / Empty State wrapped with SelectionArea spanning the entire canvas below header */}
        <SelectionArea
          selectedIds={selectedIds}
          onSelectionChange={setSelectedIds}
          className="flex-1 w-full min-h-0 overflow-auto"
        >
          {items.length === 0 ? (
            <PlaceholderView
              icon={FolderOpen}
              title="This folder is empty"
              description={
                <>
                  Right-click anywhere to create a new folder, or upload files directly into{" "}
                  <span className="font-semibold text-foreground">{item.name}</span>.
                </>
              }
            />
          ) : viewMode === "list" ? (
            <FolderContentListView
              items={items}
              selectedIds={selectedIds}
              onSelectionChange={setSelectedIds}
              onItemClick={handleItemClick}
              onItemDoubleClick={handleItemDoubleClick}
              onItemContextMenu={handleItemContextMenu}
              sortField={sortField}
              sortDirection={sortDirection}
              onSortChange={handleSortChange}
            />
          ) : (
            <FolderContentGridView
              items={items}
              selectedIds={selectedIds}
              onSelectionChange={setSelectedIds}
              onItemClick={handleItemClick}
              onItemDoubleClick={handleItemDoubleClick}
              onItemContextMenu={handleItemContextMenu}
              sortField={sortField}
              sortDirection={sortDirection}
              onSortChange={handleSortChange}
            />
          )}
        </SelectionArea>
      </div>
    </FolderContextMenu>
  )
}
