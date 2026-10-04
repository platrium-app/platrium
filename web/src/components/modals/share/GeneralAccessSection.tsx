import * as React from "react"
import { Building2, Globe, Link2, Lock } from "lucide-react"

import { NativeSelect } from "@/components/ui/native-select"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { RoleSelect, type RoleChoice } from "./RoleSelect"
import { endOfDayIso, isoToDateInput } from "./shareUtils"

export type AccessLevel = "RESTRICTED" | "TENANT" | "PUBLIC"

export interface GeneralAccessValue {
  level: string
  role?: string | null
  noDownload: boolean
  expiresAt?: string | null
}

export interface GeneralAccessChange {
  level: AccessLevel
  role?: string
  noDownload?: boolean
  expiresAt?: string | null
}

interface GeneralAccessSectionProps {
  value: GeneralAccessValue
  /** Roles the item offers; only Viewer and Editor are usable for general access. */
  roles: RoleChoice[]
  canManage: boolean
  busy: boolean
  onChange: (change: GeneralAccessChange) => void
  onCopyLink: () => void
  copied: boolean
}

const LEVELS: {
  level: AccessLevel
  label: string
  blurb: string
  icon: typeof Lock
}[] = [
  {
    level: "RESTRICTED",
    label: "Restricted",
    blurb: "Only the people added above can open this.",
    icon: Lock,
  },
  {
    level: "TENANT",
    label: "Anyone in your organization",
    blurb: "Everyone in your organization can open this.",
    icon: Building2,
  },
  {
    level: "PUBLIC",
    label: "Anyone with the link",
    blurb: "Anyone who has the link can view this, signed in or not.",
    icon: Globe,
  },
]

const GENERAL_ROLES = new Set(["VIEWER", "FULL_EDITOR"])

/** "General access": restricted, the whole organization, or anyone with the link. */
export function GeneralAccessSection({
  value,
  roles,
  canManage,
  busy,
  onChange,
  onCopyLink,
  copied,
}: GeneralAccessSectionProps) {
  const level = (
    LEVELS.some((l) => l.level === value.level) ? value.level : "RESTRICTED"
  ) as AccessLevel
  const current = LEVELS.find((l) => l.level === level)!
  const Icon = current.icon

  // Expiry must be in the future: tomorrow is the earliest day offered. Computed once.
  const [earliestExpiry] = React.useState(() =>
    isoToDateInput(new Date(Date.now() + 86_400_000).toISOString())
  )

  const orgRoles = roles.filter((r) => GENERAL_ROLES.has(r.role))
  const role = value.role ?? "VIEWER"
  const isViewer = role === "VIEWER"

  const apply = (change: Partial<GeneralAccessChange>) =>
    onChange({
      level,
      role,
      noDownload: value.noDownload,
      expiresAt: value.expiresAt ?? null,
      ...change,
    })

  const changeLevel = (next: AccessLevel) => {
    if (next === "RESTRICTED") return onChange({ level: next })
    // Public is view-only; keep an organization-wide Editor role when moving between levels only if it applies.
    const nextRole =
      next === "PUBLIC" || !GENERAL_ROLES.has(role) ? "VIEWER" : role
    onChange({
      level: next,
      role: nextRole,
      noDownload: value.noDownload,
      expiresAt: value.expiresAt ?? null,
    })
  }

  return (
    <section className="flex flex-col gap-2" aria-label="General access">
      <h3 className="text-sm font-medium">General access</h3>
      <div className="flex items-start gap-3">
        <div className="mt-1 flex size-8 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <Icon className="size-4" />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <NativeSelect
              aria-label="Who can open this"
              value={level}
              disabled={!canManage || busy}
              onChange={(e) => changeLevel(e.target.value as AccessLevel)}
            >
              {LEVELS.map((l) => (
                <option key={l.level} value={l.level}>
                  {l.label}
                </option>
              ))}
            </NativeSelect>

            {level === "TENANT" && (
              <RoleSelect
                label="Role for everyone in your organization"
                value={role}
                choices={orgRoles}
                disabled={!canManage || busy}
                onChange={(r) =>
                  apply({
                    role: r,
                    noDownload: r === "VIEWER" ? value.noDownload : false,
                  })
                }
              />
            )}
            {level === "PUBLIC" && (
              <span className="text-sm text-muted-foreground">Viewer</span>
            )}
          </div>
          <p className="text-xs text-muted-foreground">{current.blurb}</p>

          {level !== "RESTRICTED" && (
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
              {isViewer && (
                <label className="flex items-center gap-2 text-xs">
                  <input
                    type="checkbox"
                    className="size-3.5 accent-current"
                    checked={!value.noDownload}
                    disabled={!canManage || busy}
                    onChange={(e) => apply({ noDownload: !e.target.checked })}
                  />
                  Allow downloading
                </label>
              )}
              <label className="flex items-center gap-2 text-xs">
                Expires
                <Input
                  type="date"
                  aria-label="Expiry date"
                  className="h-8 w-40 text-xs"
                  value={isoToDateInput(value.expiresAt)}
                  disabled={!canManage || busy}
                  min={earliestExpiry}
                  onChange={(e) =>
                    apply({ expiresAt: endOfDayIso(e.target.value) })
                  }
                />
              </label>
            </div>
          )}
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0 gap-1.5"
          onClick={onCopyLink}
        >
          <Link2 className="size-3.5" />
          {copied ? "Copied" : "Copy link"}
        </Button>
      </div>
    </section>
  )
}
