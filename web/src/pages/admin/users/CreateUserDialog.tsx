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
import { CREATE_LOCAL_USER } from "./adminUserQueries"

const DEFAULT_ROLE = "MEMBER"

/** Creates a user who signs in to Platrium with an email and password. */
export function CreateUserDialog({
  onClose,
  assignableRoles,
}: {
  onClose: () => void
  /** Roles the signed-in admin may hand out, beyond the default. */
  assignableRoles: string[]
}) {
  const [createUser, { loading }] = useMutation(CREATE_LOCAL_USER, {
    refetchQueries: ["AdminUsers"],
    awaitRefetchQueries: true,
  })
  const [email, setEmail] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [password, setPassword] = useState("")
  const [role, setRole] = useState(DEFAULT_ROLE)
  const [error, setError] = useState<string | null>(null)

  const roles = [
    DEFAULT_ROLE,
    ...assignableRoles.filter((r) => r !== DEFAULT_ROLE),
  ]

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await createUser({
        variables: { input: { email, displayName, password, role } },
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
          <DialogTitle>Create user</DialogTitle>
          <DialogDescription>
            They sign in to Platrium with this email and password. The email
            can't be changed later.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-user-name">Name</Label>
            <Input
              id="create-user-name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              autoFocus
              required
              disabled={loading}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-user-email">Email</Label>
            <Input
              id="create-user-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="off"
              required
              disabled={loading}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-user-password">Password</Label>
            <Input
              id="create-user-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              minLength={8}
              required
              disabled={loading}
            />
            <p className="text-xs text-muted-foreground">
              At least 8 characters.
            </p>
          </div>
          {roles.length > 1 && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="create-user-role">Role</Label>
              <NativeSelect
                id="create-user-role"
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
              Create user
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
