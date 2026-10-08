import * as React from "react"
import { useVirtualizer } from "@tanstack/react-virtual"
import {
  KeyRound,
  MoreHorizontal,
  Pencil,
  UserCheck,
  UserX,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { Permission } from "@/graphql/graphql"
import { cn } from "@/lib/utils"
import { roleLabel } from "@/lib/roles"
import { SourceBadge } from "./SourceBadge"
import type { AdminUserNode } from "./types"

export type UserAction = "edit" | "reset-password" | "disable" | "enable"

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, { dateStyle: "medium" })
}

/** The actions available on a row. What the server would refuse is not offered. */
function RowMenu({
  user,
  isSelf,
  can,
  onAction,
}: {
  user: AdminUserNode
  isSelf: boolean
  can: (permission: Permission) => boolean
  onAction: (action: UserAction, user: AdminUserNode) => void
}) {
  const canEdit = user.editable && user.manageable && can("USERS_UPDATE")
  const canToggle =
    user.manageable && can("USERS_DISABLE") && !(isSelf && !user.disabled)

  if (!canEdit && !canToggle && user.editable) return null

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            className="size-7 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 data-[popup-open]:opacity-100"
            aria-label={`Actions for ${user.displayName}`}
          />
        }
      >
        <MoreHorizontal className="size-4 text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48">
        {canEdit && (
          <>
            <DropdownMenuItem onClick={() => onAction("edit", user)}>
              <Pencil />
              Edit
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => onAction("reset-password", user)}>
              <KeyRound />
              Reset password
            </DropdownMenuItem>
          </>
        )}
        {!user.editable && (
          <DropdownMenuGroup>
            <DropdownMenuLabel className="max-w-64 font-normal whitespace-normal text-muted-foreground">
              Managed by {user.source.name}. Edit this user there.
            </DropdownMenuLabel>
          </DropdownMenuGroup>
        )}
        {canToggle && (canEdit || !user.editable) && <DropdownMenuSeparator />}
        {canToggle &&
          (user.disabled ? (
            <DropdownMenuItem onClick={() => onAction("enable", user)}>
              <UserCheck />
              Enable
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem onClick={() => onAction("disable", user)}>
              <UserX />
              Disable
            </DropdownMenuItem>
          ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function UsersTable({
  users,
  selfId,
  can,
  onAction,
}: {
  users: AdminUserNode[]
  selfId?: string
  can: (permission: Permission) => boolean
  onAction: (action: UserAction, user: AdminUserNode) => void
}) {
  const parentRef = React.useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: users.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 56,
    overscan: 10,
  })

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
      <div className="grid grid-cols-12 gap-2 border-b px-4 py-3 text-sm font-medium text-muted-foreground select-none">
        <div className="col-span-8 sm:col-span-4">User</div>
        <div className="col-span-2 hidden sm:block">Source</div>
        <div className="col-span-2 hidden md:block">Role</div>
        <div className="col-span-1 hidden md:block">Status</div>
        <div className="col-span-2 hidden lg:block">Created</div>
        <div className="col-span-1" />
      </div>

      <div ref={parentRef} className="w-full flex-1 overflow-auto">
        <div
          style={{
            height: `${virtualizer.getTotalSize()}px`,
            width: "100%",
            position: "relative",
          }}
        >
          {virtualizer.getVirtualItems().map((row) => {
            const user = users[row.index]
            return (
              <div
                key={user.id}
                data-index={row.index}
                ref={virtualizer.measureElement}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${row.start}px)`,
                }}
                className={cn(
                  "group grid grid-cols-12 items-center gap-2 border-b px-4 py-3 text-sm text-foreground/90 transition-colors hover:bg-muted/50",
                  user.disabled && "text-muted-foreground"
                )}
              >
                <div className="col-span-8 flex min-w-0 items-center gap-2 sm:col-span-4">
                  <div className="min-w-0">
                    <div className="truncate font-medium">
                      {user.displayName}
                    </div>
                    <div className="truncate text-xs text-muted-foreground">
                      {user.email}
                    </div>
                  </div>
                  {user.id === selfId && <Badge variant="secondary">You</Badge>}
                </div>
                <div className="col-span-2 hidden min-w-0 sm:block">
                  <SourceBadge source={user.source} />
                </div>
                <div className="col-span-2 hidden truncate md:block">
                  {roleLabel(user.role)}
                </div>
                <div className="col-span-1 hidden md:block">
                  {user.disabled ? (
                    <Badge variant="destructive">Disabled</Badge>
                  ) : (
                    <Badge variant="secondary">Active</Badge>
                  )}
                </div>
                <div className="col-span-2 hidden truncate text-xs text-muted-foreground lg:block">
                  {formatDate(user.createdAt as string)}
                </div>
                <div className="col-span-4 flex items-center justify-end sm:col-span-1">
                  <RowMenu
                    user={user}
                    isSelf={user.id === selfId}
                    can={can}
                    onAction={onAction}
                  />
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
