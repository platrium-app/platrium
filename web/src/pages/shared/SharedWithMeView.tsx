import * as React from "react"
import { useNavigate } from "react-router-dom"
import { useQuery } from "@apollo/client/react"
import { AlertTriangle, Users } from "lucide-react"
import { graphql } from "@/graphql"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { SelectionArea } from "@/components/custom/SelectionArea"
import {
  DriveOperationManager,
  type OperationMode,
} from "@/components/modals/DriveOperationManager"
import { Spinner } from "@/components/ui/spinner"
import { useSetBreadcrumbs } from "@/contexts/BreadcrumbContext"
import { useDriveEventSubscription } from "@/hooks/useDriveEventSubscription"
import { roleLabel } from "@/lib/roles"
import { FilePreviewDialog } from "../folder/FilePreviewDialog"
import { FolderContentGridView } from "../folder/FolderContentGridView"
import { FolderContentListView } from "../folder/FolderContentListView"
import { FolderHeaderToolbar } from "../folder/FolderHeaderToolbar"
import type { DriveItemNode, ViewMode } from "../folder/FolderViewTypes"
import { useItemSelection, useItemSort } from "../folder/itemListState"

const GET_SHARED_WITH_ME = graphql(`
  query GetSharedWithMe($first: Int, $after: String) {
    sharedWithMe(first: $first, after: $after) {
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        cursor
        node {
          role
          item {
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
  }
`)

const PAGE_SIZE = 100

/** Files and folders other people have shared with the signed-in user or their groups. */
export default function SharedWithMeView() {
  useSetBreadcrumbs([{ label: "Shared with me", icon: Users }])
  const navigate = useNavigate()

  const [viewMode, setViewMode] = React.useState<ViewMode>("list")
  const [previewFileId, setPreviewFileId] = React.useState<string | null>(null)
  const [opMode, setOpMode] = React.useState<OperationMode>(null)
  const [opItems, setOpItems] = React.useState<DriveItemNode[]>([])

  const { data, loading, error, fetchMore, refetch } = useQuery(
    GET_SHARED_WITH_ME,
    {
      variables: { first: PAGE_SIZE },
    }
  )

  // The list is shown whole, so keep reading pages until there are no more.
  const connection = data?.sharedWithMe
  const nextCursor = connection?.pageInfo.hasNextPage
    ? connection.pageInfo.endCursor
    : null
  React.useEffect(() => {
    if (nextCursor) fetchMore({ variables: { after: nextCursor } })
  }, [nextCursor, fetchMore])

  // Shares, unshares and moves anywhere change what this page shows.
  useDriveEventSubscription(() => {
    refetch()
  }, true)

  const rawItems = React.useMemo<DriveItemNode[]>(
    () =>
      (connection?.edges ?? []).map(({ node }) => {
        const item = node.item
        return {
          id: item.id,
          parentId: item.parentId ?? null,
          name: item.name,
          type: item.type === "FILE" ? "FILE" : "FOLDER",
          size: "size" in item && item.size != null ? Number(item.size) : null,
          mimeType: "mimeType" in item ? (item.mimeType ?? null) : null,
          createdAt: item.createdAt as string,
          updatedAt: item.updatedAt as string,
          // The API does not say who shared an item yet, so the column shows how.
          owner: roleLabel(node.role),
          capabilities: item.myCapabilities ?? [],
        }
      }),
    [connection]
  )

  const { items, sortField, sortDirection, handleSortChange } =
    useItemSort(rawItems)
  const {
    selectedIds,
    setSelectedIds,
    clearSelection,
    handleItemClick,
    handleItemContextMenu,
  } = useItemSelection(items)

  const handleItemDoubleClick = (item: DriveItemNode) => {
    if (item.type === "FOLDER") navigate(`/folder/${item.id}`)
    else setPreviewFileId(item.id)
  }
  const handleOperation = (mode: OperationMode, ops: DriveItemNode[]) => {
    setOpMode(mode)
    setOpItems(ops)
  }

  if (loading && !data) {
    return (
      <div className="flex h-full min-h-[50vh] w-full flex-col items-center justify-center p-8 text-center">
        <Spinner className="size-8 text-muted-foreground" />
      </div>
    )
  }
  if (error) {
    return (
      <PlaceholderView
        icon={AlertTriangle}
        variant="error"
        title="Couldn't load shared items"
        description={error.message}
      />
    )
  }

  const viewProps = {
    items,
    selectedIds,
    onSelectionChange: setSelectedIds,
    onItemClick: handleItemClick,
    onItemDoubleClick: handleItemDoubleClick,
    onItemContextMenu: handleItemContextMenu,
    sortField,
    sortDirection,
    onSortChange: handleSortChange,
    onOperation: handleOperation,
    ownerLabel: "Shared as",
  }

  return (
    <>
      <div className="flex h-full w-full flex-1 flex-col overflow-hidden">
        <FolderHeaderToolbar
          selectedCount={selectedIds.size}
          onClearSelection={clearSelection}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          sortField={sortField}
          sortDirection={sortDirection}
          onSortChange={handleSortChange}
          onOperation={handleOperation}
          selectedItems={items.filter((i) => selectedIds.has(i.id))}
          ownerLabel="Shared as"
        />

        <SelectionArea
          selectedIds={selectedIds}
          onSelectionChange={setSelectedIds}
          className="flex min-h-0 w-full flex-1 flex-col"
        >
          {items.length === 0 ? (
            <PlaceholderView
              icon={Users}
              title="Nothing shared with you yet"
              description="Files and folders other people share with you will appear here."
            />
          ) : viewMode === "list" ? (
            <FolderContentListView {...viewProps} />
          ) : (
            <FolderContentGridView {...viewProps} />
          )}
        </SelectionArea>
      </div>

      <FilePreviewDialog
        fileId={previewFileId}
        onClose={() => setPreviewFileId(null)}
      />
      <DriveOperationManager
        mode={opMode}
        items={opItems}
        onClose={() => setOpMode(null)}
        onSuccess={() => {
          refetch()
          clearSelection()
        }}
      />
    </>
  )
}
