import * as React from "react"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"

export interface NamePromptModalProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (name: string) => Promise<void>
  title: string
  initialValue?: string
  fieldLabel?: string
  submitLabel?: string
}

export function NamePromptModal({
  isOpen,
  onClose,
  onSubmit,
  title,
  initialValue = "",
  fieldLabel = "Name",
  submitLabel = "Save",
}: NamePromptModalProps) {
  const [value, setValue] = React.useState(initialValue)
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  React.useEffect(() => {
    if (isOpen) {
      setValue(initialValue)
      setError(null)
    }
  }, [isOpen, initialValue])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!value.trim()) {
      setError(`A name is required`)
      return
    }

    setLoading(true)
    setError(null)
    try {
      await onSubmit(value.trim())
      onClose()
    } catch (err: any) {
      setError(err.message || "An error occurred")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          <div className="flex flex-col gap-2">
            {fieldLabel && (
              <label className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                {fieldLabel}
              </label>
            )}
            <Input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={fieldLabel ? `Enter ${fieldLabel.toLowerCase()}...` : "Enter new name..."}
              autoFocus
              disabled={loading}
            />
          </div>
          {error && <p className="text-sm font-medium text-destructive">{error}</p>}
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading && <Spinner className="mr-2 size-4" />}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
