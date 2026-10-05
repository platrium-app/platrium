import * as React from "react"
import { useNavigate } from "react-router-dom"
import { useMutation, useQuery } from "@apollo/client/react"
import { BookUser, FolderRoot, Plus } from "lucide-react"

import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { NamePromptModal } from "@/components/modals/NamePromptModal"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { useSetBreadcrumbs } from "@/contexts/BreadcrumbContext"
import {
  CAN_CREATE_SHARED_DRIVE,
  CREATE_SHARED_DRIVE,
  GET_DRIVES,
} from "./driveQueries"

/** Every shared drive the signed-in user can open, with a way to start a new one. */
export default function SharedDrivesView() {
  useSetBreadcrumbs([{ label: "Shared drives", icon: BookUser }])
  const navigate = useNavigate()

  const { data, loading } = useQuery(GET_DRIVES)
  // Only admins, and members of the groups an admin allowed, can create shared drives.
  const { data: canCreate } = useQuery(CAN_CREATE_SHARED_DRIVE)
  const [createSharedDrive] = useMutation(CREATE_SHARED_DRIVE, {
    refetchQueries: [GET_DRIVES],
    awaitRefetchQueries: true,
  })
  const [newOpen, setNewOpen] = React.useState(false)

  const drives = (data?.drives ?? []).filter(
    (d) => d.driveMetadata?.driveType === "SHARED"
  )

  const handleCreate = async (name: string) => {
    const result = await createSharedDrive({ variables: { name } })
    const created = result.data?.createSharedDrive
    if (created) navigate(`/folder/${created.id}`)
  }

  const newButton = canCreate?.canCreateSharedDrive ? (
    <Button onClick={() => setNewOpen(true)}>
      <Plus className="size-4" />
      <span>Create Shared Drive</span>
    </Button>
  ) : null

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden">
      {/* With no drives, the placeholder carries the button instead. */}
      {drives.length > 0 && (
        <div className="flex h-12 w-full shrink-0 items-center pb-2">
          {newButton}
        </div>
      )}

      {loading ? (
        <div className="flex flex-1 items-center justify-center">
          <Spinner className="size-6 text-muted-foreground" />
        </div>
      ) : drives.length === 0 ? (
        <PlaceholderView
          icon={BookUser}
          title="No shared drives yet"
          description={
            canCreate?.canCreateSharedDrive
              ? "Create a shared drive to give a team one place for its files."
              : "Shared drives you are added to will appear here."
          }
          action={newButton}
        />
      ) : (
        <ul className="grid grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3">
          {drives.map((d) => (
            <li key={d.id}>
              <Button
                variant="outline"
                className="h-auto w-full justify-start gap-3 p-4"
                onClick={() => navigate(`/folder/${d.id}`)}
              >
                <FolderRoot className="size-5 text-muted-foreground" />
                <span className="truncate">{d.name}</span>
              </Button>
            </li>
          ))}
        </ul>
      )}

      <NamePromptModal
        isOpen={newOpen}
        onClose={() => setNewOpen(false)}
        onSubmit={handleCreate}
        title="New Shared Drive"
        fieldLabel="Drive Name"
        submitLabel="Create"
      />
    </div>
  )
}
