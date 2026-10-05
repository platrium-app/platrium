import { Link } from "react-router-dom"
import { Trash2, Users } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { RoleSelect, type RoleChoice } from "./RoleSelect"
import { formatExpiry, humanizeRole, initials, type Subject } from "./shareUtils"

export interface AccessGrantRow {
  id: string
  subjectType: string
  subjectId: string
  subjectName: string
  role: string
  noDownload: boolean
  expiresAt?: string | null
  /** True when the grant is to the signed-in user, as the server reports it. */
  isYou?: boolean
  /** Set on access that comes from a folder or drive above. */
  inheritedFrom?: { id?: string | null; name: string } | null
}

interface PeopleWithAccessProps {
  owner: Subject
  grants: AccessGrantRow[]
  /** Access that comes from above. Shown read-only, with where it comes from. */
  inherited: AccessGrantRow[]
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
  you,
}: {
  name: string
  detail?: React.ReactNode
  group?: boolean
  you?: boolean
}) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <Avatar>
        <AvatarFallback>
          {group ? <Users className="size-4" /> : initials(name)}
        </AvatarFallback>
      </Avatar>
      <div className="min-w-0 leading-tight">
        <div className="truncate text-sm font-medium">
          {name}
          {you ? <span className="font-normal text-muted-foreground"> (you)</span> : null}
        </div>
        {detail ? (
          <div className="truncate text-xs text-muted-foreground">{detail}</div>
        ) : null}
      </div>
    </div>
  )
}

function roleLabel(role: string, roles: RoleChoice[]): string {
  return roles.find((r) => r.role === role)?.label ?? humanizeRole(role)
}

function grantNotes(g: AccessGrantRow): (string | null)[] {
  return [
    g.subjectType === "GROUP" ? "Group" : null,
    g.noDownload ? "Can't download" : null,
    g.expiresAt ? `Expires ${formatExpiry(g.expiresAt)}` : null,
  ]
}

/** "Can't download · From Finance", with the source linked when it can be opened. */
function InheritedDetail({ grant }: { grant: AccessGrantRow }) {
  const notes = grantNotes(grant).filter(Boolean).join(" · ")
  const from = grant.inheritedFrom
  return (
    <>
      {notes ? `${notes} · ` : null}
      {from?.id ? (
        <>
          From{" "}
          <Link
            to={`/folder/${from.id}`}
            className="underline underline-offset-2 hover:text-foreground"
          >
            {from.name}
          </Link>
        </>
      ) : (
        `From ${from?.name ?? "above"}`
      )}
    </>
  )
}

/** The owner, everyone added by name, then everyone with access from above. */
export function PeopleWithAccess({
  owner,
  grants,
  inherited,
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
          you={owner.isYou}
        />
        <span className="shrink-0 pr-3 text-sm text-muted-foreground">
          Owner
        </span>
      </li>

      {grants.map((g) => {
        const busy = busyId === g.id
        // Nobody edits their own access; someone else has to.
        const editable = canManage && !g.isYou
        return (
          <li
            key={g.id}
            className="flex items-center justify-between gap-3 py-1"
          >
            <Who
              name={g.subjectName || g.subjectId}
              detail={grantNotes(g).filter(Boolean).join(" · ") || null}
              group={g.subjectType === "GROUP"}
              you={g.isYou}
            />
            <div className="flex shrink-0 items-center gap-1">
              {editable ? (
                <>
                  <RoleSelect
                    label={`Role for ${g.subjectName}`}
                    value={g.role}
                    choices={roles}
                    disabled={busy}
                    onChange={(role) => onChangeRole(g, role)}
                  />
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
                </>
              ) : (
                <span className="pr-3 text-sm text-muted-foreground">
                  {roleLabel(g.role, roles)}
                </span>
              )}
            </div>
          </li>
        )
      })}

      {inherited.map((g) => (
        <li
          key={`inherited-${g.id}`}
          className="flex items-center justify-between gap-3 py-1"
        >
          <Who
            name={g.subjectName || g.subjectId}
            detail={<InheritedDetail grant={g} />}
            group={g.subjectType === "GROUP" || g.subjectType === "TENANT"}
            you={g.isYou}
          />
          <span className="shrink-0 pr-3 text-sm text-muted-foreground">
            {roleLabel(g.role, roles)}
          </span>
        </li>
      ))}
    </ul>
  )
}
