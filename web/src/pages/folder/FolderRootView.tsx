import * as React from "react"
import { useParams, useNavigate, useLocation } from "react-router-dom"
import { FolderOpen, AlertTriangle, FolderRoot, Folder } from "lucide-react"
import { FolderContextMenu } from "./FolderContextMenu"
import { useSetBreadcrumbs, type BreadcrumbItemType } from "@/contexts/BreadcrumbContext"
import { useQuery } from "@apollo/client/react"
import { useAuth } from "@/contexts/AuthContext"
import { Button } from "@/components/ui/button"
import { graphql } from "@/graphql"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { Spinner } from "@/components/ui/spinner"
import { FolderHeaderToolbar } from "./FolderHeaderToolbar"
import { FolderContentListView } from "./FolderContentListView"
import { FolderContentGridView } from "./FolderContentGridView"
import { SelectionArea } from "@/components/custom/SelectionArea"
import { FilePreviewDialog } from "./FilePreviewDialog"
import { useItemSelection, useItemSort } from "./itemListState"
import type { DriveItemNode, ViewMode } from "./FolderViewTypes"
import { DriveOperationManager, type OperationMode } from "@/components/modals/DriveOperationManager"
import { useDriveEventSubscription } from "@/hooks/useDriveEventSubscription"
import { hasCapability } from "@/lib/capabilities"

const GET_FOLDER_INFO = graphql(`
  query GetFolderInfo($id: ID!) {
    item(id: $id) {
      id
      parentId
      name
      type
      myCapabilities
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
          myCapabilities
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
  const location = useLocation()
  // Visitors who are not signed in can still open what is shared publicly.
  const { user } = useAuth()

  const [viewMode, setViewMode] = React.useState<ViewMode>("list")
  const [previewFileId, setPreviewFileId] = React.useState<string | null>(null)

  const [opMode, setOpMode] = React.useState<OperationMode>(null)
  const [opItems, setOpItems] = React.useState<DriveItemNode[]>([])

  const infoQuery = useQuery(GET_FOLDER_INFO, {
    variables: { id: id! },
    skip: !id,
  })

  const contentsQuery = useQuery(GET_FOLDER_CONTENTS, {
    variables: { folderId: id!, first: 100 },
    skip: !id,
  })

  // Real-time synchronization
  useDriveEventSubscription(() => {
    // When a file is modified/created/moved anywhere, we check if we need to refresh.
    // Apollo merges the new data cleanly without jumping scroll position.
    contentsQuery.refetch()
    infoQuery.refetch()
  }, !!user)

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
        capabilities: node.myCapabilities ?? [],
      }
    })
  }, [contentsQuery.data])

  const { items, sortField, sortDirection, handleSortChange } = useItemSort(rawItems)
  const {
    selectedIds,
    setSelectedIds,
    clearSelection,
    handleItemClick,
    handleItemContextMenu,
  } = useItemSelection(items)

  // Clear selection on folder navigation
  React.useEffect(() => {
    clearSelection()
  }, [id, clearSelection])

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

  const handleItemDoubleClick = (clickedItem: DriveItemNode) => {
    if (clickedItem.type === "FOLDER") {
      navigate(`/folder/${clickedItem.id}`)
    } else if (clickedItem.type === "FILE") {
      setPreviewFileId(clickedItem.id)
    }
  }

  const isLoading = 
    (infoQuery.loading && !infoQuery.data) || 
    (contentsQuery.loading && !contentsQuery.data)
  const isError = infoQuery.error || contentsQuery.error

  if (isLoading) {
    return (
      <div className="flex h-full min-h-[50vh] w-full flex-col items-center justify-center p-8 text-center animate-in fade-in duration-300">
        <Spinner className="size-8 text-muted-foreground" />
        <p className="mt-3 text-sm text-muted-foreground">Loading this Resource</p>
      </div>
    )
  }

  if (isError && !user) {
    // A signed-out visitor to a private folder: it may be theirs once they sign in.
    return (
      <PlaceholderView
        icon={AlertTriangle}
        title="This folder isn't available"
        description="It may be private. If someone shared it with you, sign in to open it."
        action={
          <Button onClick={() => navigate("/login", { state: { from: location } })}>Sign in</Button>
        }
      />
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

  if (item.type === "FILE") {
    return (
      <PlaceholderView
        icon={AlertTriangle}
        variant="error"
        title="Invalid Folder"
        description="The requested resource is a file, not a folder."
      />
    )
  }

  return (
    <FolderContextMenu 
      folderId={id!} 
      canCreate={hasCapability(item.myCapabilities, "CREATE")}
      onOperation={(mode, items) => {
        setOpMode(mode)
        setOpItems(items)
      }}
    >
      <div className="flex h-full w-full flex-1 flex-col overflow-hidden">
        {/* Permanent Toolbar */}
        <FolderHeaderToolbar
          selectedCount={selectedIds.size}
          onClearSelection={clearSelection}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          sortField={sortField}
          sortDirection={sortDirection}
          onSortChange={handleSortChange}
          folderId={id!}
          onOperation={(mode, items) => {
            setOpMode(mode)
            setOpItems(items)
          }}
          selectedItems={items.filter(i => selectedIds.has(i.id))}
          folderCapabilities={item.myCapabilities}
          currentFolder={{
            id: item.id,
            parentId: item.parentId ?? null,
            name: item.name,
            type: "FOLDER",
            createdAt: "",
            updatedAt: "",
            capabilities: item.myCapabilities,
          }}
        />

        {/* Content View / Empty State wrapped with SelectionArea spanning the entire canvas below header */}
        <SelectionArea
          selectedIds={selectedIds}
          onSelectionChange={setSelectedIds}
          className="flex-1 w-full min-h-0 flex flex-col"
        >
          {items.length === 0 ? (
            <PlaceholderView
              icon={FolderOpen}
              title="This folder is empty"
              description={
                hasCapability(item.myCapabilities, "CREATE") ? (
                  <>
                    Right-click anywhere to create a new folder, or upload files directly into{" "}
                    <span className="font-semibold text-foreground">{item.name}</span>.
                  </>
                ) : (
                  <>Nothing has been added to this folder yet.</>
                )
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
              onOperation={(mode, items) => {
                setOpMode(mode)
                setOpItems(items)
              }}
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
              onOperation={(mode, items) => {
                setOpMode(mode)
                setOpItems(items)
              }}
            />
          )}
        </SelectionArea>
      </div>

      <FilePreviewDialog fileId={previewFileId} onClose={() => setPreviewFileId(null)} />

      <DriveOperationManager
        mode={opMode}
        items={opItems}
        currentFolderId={id!}
        onClose={() => setOpMode(null)}
        onSuccess={() => {
          infoQuery.refetch()
          contentsQuery.refetch()
          clearSelection()
        }}
      />
    </FolderContextMenu>
  )
}


