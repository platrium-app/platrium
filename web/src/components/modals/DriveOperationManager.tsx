import { useMutation } from "@apollo/client/react"
import { graphql } from "@/graphql"
import type { DriveItemNode } from "@/pages/folder/FolderViewTypes"
import { NamePromptModal } from "./NamePromptModal"
import { TargetPickerModal } from "./TargetPickerModal"

const CREATE_FOLDER = graphql(`
  mutation CreateFolder($parentId: ID!, $name: String!) {
    createFolder(parentId: $parentId, name: $name) {
      id
      name
    }
  }
`)

const RENAME_ITEM = graphql(`
  mutation RenameItem($id: ID!, $newName: String!) {
    renameItem(id: $id, newName: $newName) {
      id
      name
    }
  }
`)

const MOVE_ITEM = graphql(`
  mutation MoveItem($id: ID!, $newParentId: ID!) {
    moveItem(id: $id, newParentId: $newParentId) {
      id
      name
      parentId
    }
  }
`)

const COPY_FILE = graphql(`
  mutation CopyFile($fileId: ID!, $newParentId: ID!, $newName: String!) {
    copyFile(fileId: $fileId, newParentId: $newParentId, newName: $newName) {
      id
      name
      parentId
    }
  }
`)

export type OperationMode = "CREATE_FOLDER" | "RENAME" | "MOVE" | "COPY" | null

export interface DriveOperationManagerProps {
  mode: OperationMode
  onClose: () => void
  onSuccess?: () => void
  items?: DriveItemNode[] // Required for Rename, Move, Copy
  currentFolderId: string
}

export function DriveOperationManager({
  mode,
  onClose,
  onSuccess,
  items = [],
  currentFolderId,
}: DriveOperationManagerProps) {
  const [createFolder] = useMutation(CREATE_FOLDER)
  const [renameItem] = useMutation(RENAME_ITEM)
  const [moveItem] = useMutation(MOVE_ITEM)
  const [copyFile] = useMutation(COPY_FILE)

  // 1. Create Folder Handler
  const handleCreateFolder = async (name: string) => {
    await createFolder({ variables: { parentId: currentFolderId, name } })
    onSuccess?.()
  }

  // 2. Rename Handler
  const handleRename = async (newName: string) => {
    if (items.length !== 1) throw new Error("Please select exactly one item to rename")
    await renameItem({ variables: { id: items[0].id, newName } })
    onSuccess?.()
  }

  // 3. Move Handler
  const handleMove = async (newParentId: string) => {
    if (items.length === 0) throw new Error("Please select items to move")
    await Promise.all(
      items.map((i) => moveItem({ variables: { id: i.id, newParentId } }))
    )
    onSuccess?.()
  }

  // 4. Copy Handler
  const handleCopy = async (newParentId: string) => {
    if (items.length !== 1) throw new Error("Please select exactly one file to copy")
    if (items[0].type !== "FILE") throw new Error("Only files can be copied")
    await copyFile({
      variables: {
        fileId: items[0].id,
        newParentId,
        newName: `Copy of ${items[0].name}`,
      },
    })
    onSuccess?.()
  }

  return (
    <>
      <NamePromptModal
        isOpen={mode === "CREATE_FOLDER"}
        onClose={onClose}
        onSubmit={handleCreateFolder}
        title="Create Folder"
        fieldLabel="Folder Name"
        submitLabel="Create"
      />

      <NamePromptModal
        isOpen={mode === "RENAME"}
        onClose={onClose}
        onSubmit={handleRename}
        title="Rename Item"
        fieldLabel=""
        submitLabel="Rename"
        initialValue={items.length === 1 ? items[0].name : ""}
      />

      <TargetPickerModal
        isOpen={mode === "MOVE"}
        onClose={onClose}
        onSubmit={handleMove}
        title="Move Items"
        submitLabel="Move Here"
        initialTargetId={currentFolderId}
      />

      <TargetPickerModal
        isOpen={mode === "COPY"}
        onClose={onClose}
        onSubmit={handleCopy}
        title="Make a Copy"
        submitLabel="Copy Here"
        initialTargetId={currentFolderId}
      />
    </>
  )
}
