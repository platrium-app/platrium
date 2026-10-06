import { useState, type FormEvent } from "react"
import { useMutation } from "@apollo/client/react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { NativeSelect } from "@/components/ui/native-select"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage } from "@/lib/errors"
import { roleLabel } from "@/lib/roles"
import { UPDATE_LOCAL_USER } from "./adminUserQueries"
import type { AdminUserNode } from "./types"

/** Edits a Cluster-local user's name and role. The email is their sign-in name and stays fixed. */
export function EditUserDialog({
  user,
  onClose,
  assignableRoles,
}: {
  user: AdminUserNode
  onClose: () => void
  assignableRoles: string[]
}) {
  const [updateUser, { loading }] = useMutation(UPDATE_LOCAL_USER)
  const [displayName, setDisplayName] = useState(user.displayName)
  const [role, setRole] = useState(user.role)
  const [error, setError] = useState<string | null>(null)

  // Offer the roles the admin can give, plus the user's current one so it displays.
  const roles = [user.role, ...assignableRoles.filter((r) => r !== user.role)]
  const canChangeRole = assignableRoles.length > 0

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await updateUser({
        variables: {
          id: user.id,
          input: {
            displayName,
            ...(canChangeRole && role !== user.role ? { role } : {}),
          },
        },
      })
      onClose()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !loading && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Edit user</DialogTitle>
          <DialogDescription>{user.email}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-user-name">Name</Label>
            <Input
              id="edit-user-name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              autoFocus
              required
              disabled={loading}
            />
          </div>
          {canChangeRole && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="edit-user-role">Role</Label>
              <NativeSelect
                id="edit-user-role"
                value={role}
                onChange={(e) => setRole(e.target.value)}
                disabled={loading}
              >
                {roles.map((r) => (
                  <option key={r} value={r}>
                    {roleLabel(r)}
                  </option>
                ))}
              </NativeSelect>
            </div>
          )}
          {error && (
            <p className="text-sm font-medium text-destructive">{error}</p>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading && <Spinner className="mr-2 size-4" />}
              Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
