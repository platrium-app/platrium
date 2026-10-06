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
import { Spinner } from "@/components/ui/spinner"
import { errorMessage } from "@/lib/errors"
import { RESET_LOCAL_USER_PASSWORD } from "./adminUserQueries"
import type { AdminUserNode } from "./types"

/** Sets a new password for a Cluster-local user. The old one stops working immediately. */
export function ResetPasswordDialog({
  user,
  onClose,
}: {
  user: AdminUserNode
  onClose: () => void
}) {
  const [resetPassword, { loading }] = useMutation(RESET_LOCAL_USER_PASSWORD)
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await resetPassword({ variables: { id: user.id, password } })
      onClose()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !loading && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
          <DialogDescription>
            Choose a new password for {user.displayName} ({user.email}). Tell
            them in person or over a channel you trust.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="reset-password">New password</Label>
            <Input
              id="reset-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              minLength={8}
              autoFocus
              required
              disabled={loading}
            />
            <p className="text-xs text-muted-foreground">
              At least 8 characters.
            </p>
          </div>
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
              Reset password
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
