import { useState, type FormEvent } from "react"
import { useMutation } from "@apollo/client/react"
import { dialogLayout } from "@/components/custom/dialogLayout"
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import type { UserField } from "@/graphql/graphql"
import { errorMessage } from "@/lib/errors"
import { cn } from "@/lib/utils"
import { roleLabel } from "@/lib/roles"
import { UPDATE_USER } from "./adminUserQueries"
import { SourceBadge } from "./SourceBadge"
import type { AdminUserNode } from "./types"

/**
 * Edits a user. Which fields can be changed is the server's call
 * (`editableFields`), the same for every provider: a built-in user's name is
 * Platrium's to edit, an external user's belongs to their provider, and the
 * role is Platrium's for everyone. Fields that can't be changed still show,
 * read-only, with the reason.
 */
export function EditUserDialog({
  user,
  onClose,
  assignableRoles,
}: {
  user: AdminUserNode
  onClose: () => void
  assignableRoles: string[]
}) {
  const [updateUser, { loading }] = useMutation(UPDATE_USER)
  const [displayName, setDisplayName] = useState(user.displayName)
  const [role, setRole] = useState(user.role)
  const [error, setError] = useState<string | null>(null)

  const can = (field: UserField) => user.editableFields.includes(field)
  const external = !user.source.isLocal

  // Offer the roles the admin can give, plus the user's current one so it displays.
  const roles = [user.role, ...assignableRoles.filter((r) => r !== user.role)]
  const roleItems = Object.fromEntries(roles.map((r) => [r, roleLabel(r)]))

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    // Send only what can be changed, and has been.
    const input = {
      ...(can("DISPLAY_NAME") && displayName !== user.displayName
        ? { displayName }
        : {}),
      ...(can("ROLE") && role !== user.role ? { role } : {}),
    }
    if (Object.keys(input).length === 0) return onClose()
    try {
      await updateUser({ variables: { id: user.id, input } })
      onClose()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !loading && onClose()}>
      <DialogContent className={cn(dialogLayout.content, "sm:max-w-md")}>
        <DialogHeader className={dialogLayout.header}>
          <DialogTitle>Edit user</DialogTitle>
          <DialogDescription className="flex items-center gap-2">
            Signs in with <SourceBadge source={user.source} showType />
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className={dialogLayout.form}>
          <div className={dialogLayout.body}>
            <div className="flex flex-col gap-2">
              <Label htmlFor="edit-user-name">Name</Label>
              {external && (
                <p className="text-xs text-muted-foreground">
                  Synced from {user.source.name} every time they sign in.
                </p>
              )}
              <Input
                id="edit-user-name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                disabled={loading || !can("DISPLAY_NAME")}
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="edit-user-email">Email</Label>
              <p className="text-xs text-muted-foreground">
                {external
                  ? `Synced from ${user.source.name} every time they sign in.`
                  : "Their sign-in name, so it can't be changed."}
              </p>
              <Input id="edit-user-email" value={user.email} disabled />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="edit-user-role">Role</Label>
              {!can("ROLE") && (
                <p className="text-xs text-muted-foreground">
                  You can't change this user's role.
                </p>
              )}
              <Select
                value={role}
                onValueChange={(v) => v && setRole(v)}
                items={roleItems}
                disabled={loading || !can("ROLE")}
              >
                <SelectTrigger id="edit-user-role" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {roles.map((r) => (
                      <SelectItem key={r} value={r}>
                        {roleLabel(r)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
          </div>
          {error && <p className={dialogLayout.error}>{error}</p>}
          <DialogFooter className={dialogLayout.footer}>
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
              Save changes
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
