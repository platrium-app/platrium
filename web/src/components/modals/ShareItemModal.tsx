import * as React from "react"
import { useMutation, useQuery } from "@apollo/client/react"
import { AlertTriangle, Link2 } from "lucide-react"

import { cn } from "@/lib/utils"
import { isSharedDriveRoot } from "@/lib/capabilities"
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
  type Busy,
  dialogReducer,
  initialDialogState,
  phaseOf,
} from "./share/ShareDialogState"
import {
  GENERAL_ACCESS_OPTIONS,
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
 * Opens and closes a block of content by animating its height (a grid row going
 * between 0fr and 1fr), so the dialog grows and shrinks instead of jumping.
 * Closed content is inert: not focusable and hidden from screen readers.
 */
function Collapse({
  open,
  children,
}: {
  open: boolean
  children: React.ReactNode
}) {
  return (
    <div
      inert={!open}
      className={cn(
        "grid transition-[grid-template-rows,opacity] duration-200 ease-out motion-reduce:transition-none",
        open ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"
      )}
    >
      <div className="-mx-1 min-h-0 overflow-hidden px-1">{children}</div>
    </div>
  )
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
  const levelsQuery = useQuery(GENERAL_ACCESS_OPTIONS, {
    variables: { itemId },
    fetchPolicy: "network-only",
  })

  const [shareItem] = useMutation(SHARE_ITEM)
  const [revokeAccess] = useMutation(REVOKE_ACCESS)
  const [setGeneralAccess] = useMutation(SET_GENERAL_ACCESS)
  const [setInheritance] = useMutation(SET_INHERITANCE)

  const [state, dispatch] = React.useReducer(dialogReducer, initialDialogState)
  const { recipients, expiry, busy, error, copied } = state

  const roles = React.useMemo(
    () => rolesQuery.data?.shareRoles ?? [],
    [rolesQuery.data]
  )
  const levels = React.useMemo(
    () => levelsQuery.data?.generalAccessOptions ?? [],
    [levelsQuery.data]
  )

  // Default to the first (least privileged) role the server offers, until the user picks one.
  const activeRole = roles.some((r) => r.role === state.role)
    ? (state.role as string)
    : (roles[0]?.role ?? "")

  const data = access.data?.itemAccess
  const phase = phaseOf({
    loading:
      access.loading ||
      (rolesQuery.loading && roles.length === 0) ||
      levelsQuery.loading,
    hasData: !!data && levels.length > 0,
    denied: access.error
      ? ["FORBIDDEN", "NOT_FOUND"].includes(errorCode(access.error) ?? "")
      : false,
  })
  const kind = item.type === "FOLDER" ? "folder" : "file"
  const inherited = React.useMemo<AccessGrantRow[]>(
    () =>
      (data?.inherited ?? []).map((g) => ({
        ...g,
        expiresAt: asIso(g.expiresAt),
      })),
    [data]
  )
  const grants = React.useMemo<AccessGrantRow[]>(
    () =>
      (data?.grants ?? []).map((g) => ({
        ...g,
        expiresAt: asIso(g.expiresAt),
      })),
    [data]
  )
  const idle = busy.kind === "idle"
  // Once someone is picked the dialog is about adding them: Cancel or Share, and
  // nothing else competing for attention. Sharing returns to the full view.
  const adding = recipients.length > 0

  /** Run one write, holding the dialog busy until it and the refresh finish. */
  const run = async (
    b: Exclude<Busy, { kind: "idle" }>,
    fn: () => Promise<unknown>
  ) => {
    dispatch({ type: "start", busy: b })
    try {
      await fn()
      await access.refetch()
      dispatch({ type: "done" })
    } catch (err) {
      dispatch({ type: "done", error: friendlyError(err) })
    }
  }

  const handleShare = async () => {
    if (recipients.length === 0) return
    dispatch({ type: "start", busy: { kind: "sharing" } })
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
              expiresAt: endOfDayIso(expiry),
            },
          },
        })
      } catch (err) {
        failed.push(r)
        lastError = err
      }
    }
    await access.refetch()
    dispatch({
      type: "done",
      keep: failed,
      error: failed.length > 0 ? friendlyError(lastError) : null,
    })
  }

  const handleChangeRole = (g: AccessGrantRow, role: string) =>
    run({ kind: "grant", id: g.id }, () =>
      shareItem({
        variables: {
          input: {
            itemId,
            subjectType: g.subjectType,
            subjectId: g.subjectId,
            role,
            noDownload: roles.find((r) => r.role === role)?.downloadOptional
              ? g.noDownload
              : false,
            expiresAt: g.expiresAt ?? null,
          },
        },
      })
    )

  const handleRemove = (g: AccessGrantRow) =>
    run({ kind: "grant", id: g.id }, () =>
      revokeAccess({ variables: { grantId: g.id } })
    )

  const handleGeneral = (change: GeneralAccessChange) =>
    run({ kind: "general" }, () =>
      setGeneralAccess({
        variables: {
          input: {
            itemId,
            level: change.level,
            role: change.role ?? null,
            noDownload: change.noDownload ?? false,
            expiresAt: change.expiresAt ?? null,
          },
        },
      })
    )

  const handleInheritance = (inherit: boolean) =>
    run({ kind: "inheritance" }, () =>
      setInheritance({ variables: { itemId, inherit } })
    )

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(itemLink(item.id, item.type))
      dispatch({ type: "copied", copied: true })
      setTimeout(() => dispatch({ type: "copied", copied: false }), 2000)
    } catch {
      dispatch({
        type: "error",
        message:
          "Could not copy the link. Copy it from the address bar instead.",
      })
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle className="truncate">
          {isSharedDriveRoot(item) ? "Manage access to" : "Share"} “{item.name}”
        </DialogTitle>
        <DialogDescription className="sr-only">
          Choose who can open this {kind} and what they can do.
        </DialogDescription>
      </DialogHeader>

      {phase === "loading" && (
        <div className="flex items-center justify-center py-10">
          <Spinner className="size-6 text-muted-foreground" />
        </div>
      )}

      {phase === "denied" && (
        <div className="flex items-start gap-3 rounded-2xl bg-muted/60 p-4 text-sm">
          <AlertTriangle className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <p>
            Only people who can share this {kind} can see or change who has
            access to it. Ask its owner to share it with you or to give you more
            access.
          </p>
        </div>
      )}

      {phase === "ready" && data && (
        <div className="-m-1 flex max-h-[70vh] flex-col overflow-y-auto p-1">
          <section aria-label="Add people">
            <PeoplePicker
              itemId={itemId}
              selected={recipients}
              onChange={(next) =>
                dispatch({ type: "recipients", recipients: next })
              }
              disabled={busy.kind === "sharing"}
            />
            <Collapse open={adding}>
              <div className="flex flex-wrap items-center gap-2 pt-3">
                <RoleSelect
                  label="Role for the people you are adding"
                  value={activeRole}
                  choices={roles}
                  onChange={(role) => dispatch({ type: "role", role })}
                />
                <Input
                  type="date"
                  aria-label="Expiry date for the new access"
                  className="w-auto"
                  value={expiry}
                  onChange={(e) =>
                    dispatch({ type: "expiry", expiry: e.target.value })
                  }
                />
              </div>
            </Collapse>
          </section>

          <Collapse open={!adding}>
            <div className="flex flex-col gap-5 pt-5">
              <section
                className="flex flex-col gap-2"
                aria-label="People with access"
              >
                <div className="flex flex-col gap-0.5">
                  <h3 className="text-sm font-medium">People with access</h3>
                  {grants.length === 0 && inherited.length === 0 && (
                    <p className="text-xs text-muted-foreground">
                      Only the owner can open this. Add people below to share
                      it.
                    </p>
                  )}
                </div>
                <PeopleWithAccess
                  owner={data.owner}
                  grants={grants}
                  inherited={inherited}
                  roles={roles}
                  busyId={busy.kind === "grant" ? busy.id : null}
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
                levels={levels}
                roles={roles}
                canManage
                busy={!idle}
                onChange={handleGeneral}
              />

              {item.parentId ? (
                <section
                  className="flex items-center justify-between gap-4 rounded-2xl bg-muted/40 p-4"
                  aria-label="Inherited access"
                >
                  <p className="text-sm text-muted-foreground">
                    {data.inheritsPermissions
                      ? `People with access to the folder above can also open this ${kind}.`
                      : `Only the people listed here can open this ${kind}. Access from the folder above doesn't apply.`}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    className="shrink-0"
                    disabled={!idle}
                    onClick={() => handleInheritance(!data.inheritsPermissions)}
                  >
                    {data.inheritsPermissions
                      ? "Restrict access"
                      : "Inherit access"}
                  </Button>
                </section>
              ) : null}
            </div>
          </Collapse>

          {error && (
            <p
              role="alert"
              className="mt-5 rounded-2xl bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </p>
          )}
        </div>
      )}

      <DialogFooter className="sm:justify-between">
        {adding ? (
          <>
            <span />
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={!idle}
                onClick={() => dispatch({ type: "recipients", recipients: [] })}
              >
                Cancel
              </Button>
              <Button
                type="button"
                disabled={!idle || !activeRole}
                onClick={handleShare}
              >
                {busy.kind === "sharing" ? <Spinner /> : null}
                Share
              </Button>
            </div>
          </>
        ) : (
          <>
            {phase === "ready" ? (
              <Button type="button" variant="outline" onClick={copyLink}>
                <Link2 />
                {copied ? "Copied" : "Copy link"}
              </Button>
            ) : (
              <span />
            )}
            <Button type="button" onClick={onClose}>
              Done
            </Button>
          </>
        )}
      </DialogFooter>
    </>
  )
}
