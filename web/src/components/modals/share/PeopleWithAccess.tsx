import { Trash2, Users } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { RoleSelect, type RoleChoice } from "./RoleSelect"
import { formatExpiry, initials, type Subject } from "./shareUtils"

export interface AccessGrantRow {
  id: string
  subjectType: string
  subjectId: string
  subjectName: string
  role: string
  noDownload: boolean
  expiresAt?: string | null
}

interface PeopleWithAccessProps {
  owner: Subject
  grants: AccessGrantRow[]
  roles: RoleChoice[]
  busyId: string | null
  canManage: boolean
  onChangeRole: (grant: AccessGrantRow, role: string) => void
  onRemove: (grant: AccessGrantRow) => void
}

function Who({
  name,
  detail,
  group,
}: {
  name: string
  detail?: string | null
  group?: boolean
}) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <Avatar>
        <AvatarFallback>
          {group ? <Users className="size-4" /> : initials(name)}
        </AvatarFallback>
      </Avatar>
      <div className="min-w-0 leading-tight">
        <div className="truncate text-sm font-medium">{name}</div>
        {detail ? (
          <div className="truncate text-xs text-muted-foreground">{detail}</div>
        ) : null}
      </div>
    </div>
  )
}

/** The owner, then everyone who was added by name. */
export function PeopleWithAccess({
  owner,
  grants,
  roles,
  busyId,
  canManage,
  onChangeRole,
  onRemove,
}: PeopleWithAccessProps) {
  return (
    <ul className="flex flex-col gap-1" aria-label="People with access">
      <li className="flex items-center justify-between gap-3 py-1">
        <Who
          name={owner.name}
          detail={owner.type === "TENANT" ? "Your organization" : owner.email}
          group={owner.type === "TENANT"}
        />
        <span className="shrink-0 pr-3 text-sm text-muted-foreground">
          Owner
        </span>
      </li>

      {grants.map((g) => {
        const busy = busyId === g.id
        const notes = [
          g.noDownload ? "Can't download" : null,
          g.expiresAt ? `Expires ${formatExpiry(g.expiresAt)}` : null,
        ].filter(Boolean)
        return (
          <li
            key={g.id}
            className="flex items-center justify-between gap-3 py-1"
          >
            <Who
              name={g.subjectName || g.subjectId}
              detail={
                [g.subjectType === "GROUP" ? "Group" : null, ...notes]
                  .filter(Boolean)
                  .join(" · ") || null
              }
              group={g.subjectType === "GROUP"}
            />
            <div className="flex shrink-0 items-center gap-1">
              <RoleSelect
                label={`Role for ${g.subjectName}`}
                value={g.role}
                choices={roles}
                disabled={!canManage || busy}
                onChange={(role) => onChangeRole(g, role)}
              />
              {canManage && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Remove access for ${g.subjectName}`}
                  title="Remove access"
                  disabled={busy}
                  onClick={() => onRemove(g)}
                >
                  <Trash2 className="size-4" />
                </Button>
              )}
            </div>
          </li>
        )
      })}

      {grants.length === 0 && (
        <li className="py-1 text-xs text-muted-foreground">
          Only the owner can open this. Add people above to share it.
        </li>
      )}
    </ul>
  )
}
