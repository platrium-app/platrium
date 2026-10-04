import * as React from "react"
import { useMutation, useQuery } from "@apollo/client/react"
import { AlertTriangle } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  GeneralAccessSection,
  type GeneralAccessChange,
} from "./share/GeneralAccessSection"
import { PeoplePicker } from "./share/PeoplePicker"
import { PeopleWithAccess, type AccessGrantRow } from "./share/PeopleWithAccess"
import { RoleSelect } from "./share/RoleSelect"
import {
  ITEM_ACCESS,
  REVOKE_ACCESS,
  SET_GENERAL_ACCESS,
  SET_INHERITANCE,
  SHARE_ITEM,
  SHARE_ROLES,
} from "./share/shareQueries"
import {
  asIso,
  endOfDayIso,
  errorCode,
  friendlyError,
  itemLink,
  type Subject,
} from "./share/shareUtils"

export interface ShareableItem {
  id: string
  name: string
  type: "FILE" | "FOLDER"
  /** Null for a drive's root, which has nothing to inherit. */
  parentId?: string | null
}

export interface ShareItemModalProps {
  isOpen: boolean
  onClose: () => void
  item: ShareableItem | null
}

/**
 * One dialog for sharing anything: a file, a folder, or a shared drive (whose
 * "sharing" is its membership). The server decides which roles to offer and what
 * they are called, so this component never hard-codes either.
 */
export function ShareItemModal({ isOpen, onClose, item }: ShareItemModalProps) {
  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-xl" data-testid="share-dialog">
        {/* Mounted only while open, and per item, so every opening starts with a clean form. */}
        {isOpen && item ? (
          <ShareItemBody key={item.id} item={item} onClose={onClose} />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function ShareItemBody({
  item,
  onClose,
}: {
  item: ShareableItem
  onClose: () => void
}) {
  const itemId = item.id

  const access = useQuery(ITEM_ACCESS, {
    variables: { itemId },
    fetchPolicy: "network-only",
  })
  const rolesQuery = useQuery(SHARE_ROLES, {
    variables: { itemId },
    fetchPolicy: "network-only",
  })

  const [shareItem] = useMutation(SHARE_ITEM)
  const [revokeAccess] = useMutation(REVOKE_ACCESS)
  const [setGeneralAccess] = useMutation(SET_GENERAL_ACCESS)
  const [setInheritance] = useMutation(SET_INHERITANCE)

  const [recipients, setRecipients] = React.useState<Subject[]>([])
  const [newRole, setNewRole] = React.useState("")
  const [newExpiry, setNewExpiry] = React.useState("")
  const [sharing, setSharing] = React.useState(false)
  const [busyId, setBusyId] = React.useState<string | null>(null)
  const [generalBusy, setGeneralBusy] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [copied, setCopied] = React.useState(false)

  const roles = React.useMemo(
    () => rolesQuery.data?.shareRoles ?? [],
    [rolesQuery.data]
  )

  // Default to the first (least privileged) role the server offers, until the user picks one.
  const activeRole = roles.some((r) => r.role === newRole)
    ? newRole
    : (roles[0]?.role ?? "")

  const data = access.data?.itemAccess
  const denied = access.error
    ? ["FORBIDDEN", "NOT_FOUND"].includes(errorCode(access.error) ?? "")
    : false
  const loading =
    (access.loading && !data) || (rolesQuery.loading && roles.length === 0)
  const kind = item.type === "FOLDER" ? "folder" : "file"
  const grants = React.useMemo<AccessGrantRow[]>(
    () =>
      (data?.grants ?? []).map((g) => ({
        ...g,
        expiresAt: asIso(g.expiresAt),
      })),
    [data]
  )

  const run = async (fn: () => Promise<unknown>) => {
    setError(null)
    try {
      await fn()
      await access.refetch()
    } catch (err) {
      setError(friendlyError(err))
    }
  }

  const handleShare = async () => {
    if (recipients.length === 0) return
    setSharing(true)
    setError(null)
    const failed: Subject[] = []
    let lastError: unknown = null
    for (const r of recipients) {
      try {
        await shareItem({
          variables: {
            input: {
              itemId: item.id,
              subjectType: r.type,
              subjectId: r.id,
              role: activeRole,
              noDownload: false,
              expiresAt: endOfDayIso(newExpiry),
            },
          },
        })
      } catch (err) {
        failed.push(r)
        lastError = err
      }
    }
    setSharing(false)
    setRecipients(failed) // keep only the ones that did not go through
    if (failed.length > 0) setError(friendlyError(lastError))
    else setNewExpiry("")
    await access.refetch()
  }

  const handleChangeRole = (g: AccessGrantRow, role: string) =>
    run(async () => {
      setBusyId(g.id)
      try {
        await shareItem({
          variables: {
            input: {
              itemId,
              subjectType: g.subjectType,
              subjectId: g.subjectId,
              role,
              noDownload: role === "VIEWER" ? g.noDownload : false,
              expiresAt: g.expiresAt ?? null,
            },
          },
        })
      } finally {
        setBusyId(null)
      }
    })

  const handleRemove = (g: AccessGrantRow) =>
    run(async () => {
      setBusyId(g.id)
      try {
        await revokeAccess({ variables: { grantId: g.id } })
      } finally {
        setBusyId(null)
      }
    })

  const handleGeneral = (change: GeneralAccessChange) =>
    run(async () => {
      setGeneralBusy(true)
      try {
        await setGeneralAccess({
          variables: {
            input: {
              itemId,
              level: change.level,
              role:
                change.level === "RESTRICTED"
                  ? null
                  : (change.role ?? "VIEWER"),
              noDownload: change.noDownload ?? false,
              expiresAt: change.expiresAt ?? null,
            },
          },
        })
      } finally {
        setGeneralBusy(false)
      }
    })

  const handleInheritance = (inherit: boolean) =>
    run(() => setInheritance({ variables: { itemId, inherit } }))

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(itemLink(item.id, item.type))
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      setError("Could not copy the link. Copy it from the address bar instead.")
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle className="truncate">Share “{item.name}”</DialogTitle>
        <DialogDescription className="sr-only">
          Choose who can open this {kind} and what they can do.
        </DialogDescription>
      </DialogHeader>

      {loading ? (
        <div className="flex items-center justify-center py-10">
          <Spinner className="size-6 text-muted-foreground" />
        </div>
      ) : denied || !data ? (
        <div className="flex items-start gap-3 rounded-2xl bg-muted/60 p-4 text-sm">
          <AlertTriangle className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <p>
            Only people who can share this {kind} can see or change who has
            access to it. Ask its owner to share it with you or to give you more
            access.
          </p>
        </div>
      ) : (
        <div className="flex max-h-[70vh] flex-col gap-5 overflow-y-auto pr-1">
          {/* Add people */}
          <section className="flex flex-col gap-2" aria-label="Add people">
            <PeoplePicker
              itemId={itemId}
              selected={recipients}
              onChange={setRecipients}
              disabled={sharing}
            />
            {recipients.length > 0 && (
              <div className="flex flex-wrap items-center gap-2">
                <RoleSelect
                  label="Role for the people you are adding"
                  value={activeRole}
                  choices={roles}
                  onChange={setNewRole}
                />
                <label className="flex items-center gap-2 text-xs">
                  Expires
                  <Input
                    type="date"
                    aria-label="Expiry date for the new access"
                    className="h-8 w-40 text-xs"
                    value={newExpiry}
                    onChange={(e) => setNewExpiry(e.target.value)}
                  />
                </label>
                <Button
                  type="button"
                  className="ml-auto"
                  disabled={sharing || !activeRole}
                  onClick={handleShare}
                >
                  {sharing ? <Spinner /> : null}
                  Share
                </Button>
              </div>
            )}
          </section>

          {/* People with access */}
          <section
            className="flex flex-col gap-2"
            aria-label="People with access"
          >
            <h3 className="text-sm font-medium">People with access</h3>
            <PeopleWithAccess
              owner={data.owner}
              grants={grants}
              roles={roles}
              busyId={busyId}
              canManage
              onChangeRole={handleChangeRole}
              onRemove={handleRemove}
            />
          </section>

          <GeneralAccessSection
            value={{
              ...data.generalAccess,
              expiresAt: asIso(data.generalAccess.expiresAt),
            }}
            roles={roles}
            canManage
            busy={generalBusy}
            onChange={handleGeneral}
            onCopyLink={copyLink}
            copied={copied}
          />

          {item.parentId ? (
            <section
              className="flex items-start justify-between gap-3 rounded-2xl bg-muted/40 p-3"
              aria-label="Inherited access"
            >
              <p className="text-xs text-muted-foreground">
                {data.inheritsPermissions
                  ? `People with access to the folder above can also open this ${kind}.`
                  : `Only the people listed here can open this ${kind}. Access from the folder above doesn't apply.`}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="shrink-0"
                onClick={() => handleInheritance(!data.inheritsPermissions)}
              >
                {data.inheritsPermissions
                  ? "Restrict access"
                  : "Inherit access"}
              </Button>
            </section>
          ) : null}

          {error && (
            <p
              role="alert"
              className="rounded-2xl bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </p>
          )}
        </div>
      )}

      <DialogFooter>
        <Button type="button" onClick={onClose}>
          Done
        </Button>
      </DialogFooter>
    </>
  )
}
