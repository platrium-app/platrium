import * as React from "react"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { useQuery, useApolloClient } from "@apollo/client/react"
import { graphql } from "@/graphql"
import { Folder } from "lucide-react"
import { FilesystemTree, type FileTreeNode } from "@/components/custom/FilesystemTreeView"
import { SidebarMenu } from "@/components/ui/sidebar"

const GET_DRIVES = graphql(`
  query GetDrivesForPicker {
    drives {
      id
      name
    }
  }
`)

const GET_SUBFOLDERS_PICKER = graphql(`
  query GetSubfoldersPicker($folderId: ID!) {
    folderContents(folderId: $folderId, first: 100) {
      edges {
        node {
          id
          name
          type
        }
      }
    }
  }
`)

function FolderPicker({ onSelect, selectedId }: { onSelect: (id: string) => void, selectedId: string | null }) {
  const { data } = useQuery(GET_DRIVES)
  
  const nodes: FileTreeNode[] = (data?.drives || []).map(drive => ({
    id: drive.id,
    name: drive.name,
    hasChildren: true,
    icon: Folder
  }))

  const apolloClient = useApolloClient()

  const getChildren = React.useCallback(async (parentId: string) => {
    try {
      const result = await apolloClient.query({
        query: GET_SUBFOLDERS_PICKER,
        variables: { folderId: parentId },
      })

      const children: FileTreeNode[] = []
      ;(result.data as any).folderContents?.edges?.forEach((edge: any) => {
        if (edge?.node && edge.node.type === "FOLDER") {
          children.push({
            id: edge.node.id,
            name: edge.node.name,
            hasChildren: true,
            icon: Folder
          })
        }
      })
      return children
    } catch (err) {
      console.error("Failed to fetch subfolders", err)
      return []
    }
  }, [apolloClient])

  return (
    <div className="h-48 overflow-y-auto rounded-md border bg-muted/20 p-2 mt-2">
      <SidebarMenu>
        <FilesystemTree
          nodes={nodes}
          getChildren={getChildren}
          activeId={selectedId || undefined}
          onSelect={onSelect}
        />
      </SidebarMenu>
    </div>
  )
}

export interface TargetPickerModalProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (targetId: string) => Promise<void>
  title: string
  initialTargetId?: string
  submitLabel?: string
}

export function TargetPickerModal({
  isOpen,
  onClose,
  onSubmit,
  title,
  initialTargetId = "",
  submitLabel = "Confirm",
}: TargetPickerModalProps) {
  const [selectedId, setSelectedId] = React.useState<string | null>(initialTargetId)
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  React.useEffect(() => {
    if (isOpen) {
      setSelectedId(initialTargetId)
      setError(null)
    }
  }, [isOpen, initialTargetId])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedId) {
      setError("Please select a destination folder")
      return
    }

    setLoading(true)
    setError(null)
    try {
      await onSubmit(selectedId)
      onClose()
    } catch (err: any) {
      setError(err.message || "An error occurred")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          <div className="flex flex-col">
            <FolderPicker selectedId={selectedId} onSelect={setSelectedId} />
          </div>
          {error && <p className="text-sm font-medium text-destructive">{error}</p>}
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading || !selectedId}>
              {loading && <Spinner className="mr-2 size-4" />}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
