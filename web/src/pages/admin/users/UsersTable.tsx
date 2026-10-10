import * as React from "react"
import { useVirtualizer } from "@tanstack/react-virtual"
import {
  KeyRound,
  MoreHorizontal,
  Pencil,
  Trash2,
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

// The grid is 12 columns. At each width the visible columns add up to 11 and the
// menu always takes the last one, so it stays at the right edge however many
// of the others are hidden.
const COL = {
  user: "col-span-8 sm:col-span-7 md:col-span-5 lg:col-span-4",
  source: "hidden sm:col-span-4 sm:block md:col-span-3 lg:col-span-2",
  role: "hidden md:col-span-2 md:block",
  status: "hidden md:col-span-1 md:block",
  created: "hidden lg:col-span-2 lg:block",
  menu: "col-span-4 sm:col-span-1 sm:col-start-12",
}

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
  // What can be edited is decided by the server, the same for every provider.
  const canEdit =
    user.editableFields.includes("DISPLAY_NAME") ||
    user.editableFields.includes("ROLE")
  const canResetPassword = user.editableFields.includes("PASSWORD")
  const canToggle =
    user.manageable && can("USERS_DISABLE") && !(isSelf && !user.disabled)

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
          <DropdownMenuItem onClick={() => onAction("edit", user)}>
            <Pencil />
            Edit
          </DropdownMenuItem>
        )}
        {canResetPassword && (
          <DropdownMenuItem onClick={() => onAction("reset-password", user)}>
            <KeyRound />
            Reset password
          </DropdownMenuItem>
        )}
        {!canEdit && !user.source.isLocal && (
          <DropdownMenuGroup>
            <DropdownMenuLabel className="max-w-64 font-normal whitespace-normal text-muted-foreground">
              Managed by {user.source.name}.
            </DropdownMenuLabel>
          </DropdownMenuGroup>
        )}
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
        <DropdownMenuSeparator />
        {/* Not built yet. It shows so nobody wonders where it is. */}
        <DropdownMenuItem variant="destructive" disabled>
          <Trash2 />
          Delete
          <span className="ml-auto text-xs text-muted-foreground">
            Coming soon
          </span>
        </DropdownMenuItem>
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
        <div className={COL.user}>User</div>
        <div className={COL.source}>Source</div>
        <div className={COL.role}>Role</div>
        <div className={COL.status}>Status</div>
        <div className={COL.created}>Created</div>
        <div className={COL.menu} />
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
                <div
                  className={cn("flex min-w-0 items-center gap-2", COL.user)}
                >
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
                <div className={cn("min-w-0", COL.source)}>
                  <SourceBadge source={user.source} />
                </div>
                <div className={cn("truncate", COL.role)}>
                  {roleLabel(user.role)}
                </div>
                <div className={COL.status}>
                  {user.disabled ? (
                    <Badge variant="destructive">Disabled</Badge>
                  ) : (
                    <Badge variant="secondary">Active</Badge>
                  )}
                </div>
                <div
                  className={cn(
                    "truncate text-xs text-muted-foreground",
                    COL.created
                  )}
                >
                  {formatDate(user.createdAt as string)}
                </div>
                <div className={cn("flex items-center justify-end", COL.menu)}>
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
