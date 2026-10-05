import * as React from "react"
import { Building2, Globe, Lock, Settings2, type LucideIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { OptionMenu } from "./OptionMenu"
import type { RoleChoice } from "./RoleSelect"
import {
  endOfDayIso,
  formatExpiry,
  humanizeRole,
  isoToDateInput,
} from "./shareUtils"

/** A level as the server describes it (see generalAccessOptions). */
export interface AccessLevelChoice {
  level: string
  label: string
  blurb: string
  roles: string[]
  supportsExpiry: boolean
}

export interface GeneralAccessValue {
  level: string
  role?: string | null
  noDownload: boolean
  expiresAt?: string | null
}

export interface GeneralAccessChange {
  level: string
  role?: string
  noDownload?: boolean
  expiresAt?: string | null
}

interface GeneralAccessSectionProps {
  value: GeneralAccessValue
  /** The levels the server offers for this item. */
  levels: AccessLevelChoice[]
  /** Every role the item offers; each level names the ones it takes. */
  roles: RoleChoice[]
  canManage: boolean
  busy: boolean
  onChange: (change: GeneralAccessChange) => void
}

// Only presentation lives here. What a level allows comes from the server.
const ICONS: Record<string, LucideIcon> = {
  RESTRICTED: Lock,
  TENANT: Building2,
  PUBLIC: Globe,
}

/** "General access": who else can open this, beyond the people added by name. */
export function GeneralAccessSection({
  value,
  levels,
  roles,
  canManage,
  busy,
  onChange,
}: GeneralAccessSectionProps) {
  // A level set earlier but no longer offered (a policy changed) still shows.
  const current: AccessLevelChoice = levels.find((l) => l.level === value.level) ?? {
    level: value.level,
    label: humanizeRole(value.level),
    blurb: "",
    roles: value.role ? [value.role] : [],
    supportsExpiry: !!value.expiresAt,
  }
  const options = levels.some((l) => l.level === current.level)
    ? levels
    : [current, ...levels]

  const takesRole = current.roles.length > 0
  const role = value.role ?? current.roles[0]
  const roleChoices = current.roles.map(
    (r) => roles.find((c) => c.role === r) ?? { role: r, label: humanizeRole(r) }
  )
  const downloadOptional = roles.find((r) => r.role === role)?.downloadOptional
  const hasSettings = takesRole && (downloadOptional || current.supportsExpiry)

  const apply = (change: Partial<GeneralAccessChange>) =>
    onChange({
      level: current.level,
      role,
      noDownload: value.noDownload,
      expiresAt: value.expiresAt ?? null,
      ...change,
    })

  const changeLevel = (next: string) => {
    const def = options.find((l) => l.level === next)
    if (!def || def.roles.length === 0) return onChange({ level: next })
    const nextRole = role && def.roles.includes(role) ? role : def.roles[0]
    const keepsDownload = roles.find((r) => r.role === nextRole)?.downloadOptional
    onChange({
      level: next,
      role: nextRole,
      noDownload: keepsDownload ? value.noDownload : false,
      expiresAt: def.supportsExpiry ? (value.expiresAt ?? null) : null,
    })
  }

  const notes = [
    takesRole && downloadOptional && value.noDownload ? "Downloading is off" : null,
    value.expiresAt && current.supportsExpiry
      ? `Expires ${formatExpiry(value.expiresAt)}`
      : null,
  ].filter(Boolean)

  // Expiry must be in the future: tomorrow is the earliest day offered.
  const [tomorrow] = React.useState(() =>
    isoToDateInput(new Date(Date.now() + 86_400_000).toISOString())
  )

  return (
    <section className="flex flex-col gap-1" aria-label="General access">
      <h3 className="text-sm font-medium">General access</h3>
      <p className="text-xs text-muted-foreground">
        {[current.blurb, ...notes].filter(Boolean).join(" · ")}
      </p>
      <div className="flex items-center gap-3 mt-2">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex flex-wrap items-center gap-2">
            <OptionMenu
              label="Who can open this"
              value={current.level}
              wide
              options={options.map((l) => ({ value: l.level, label: l.label, icon: ICONS[l.level] ?? Lock }))}
              disabled={!canManage || busy}
              onChange={changeLevel}
            />
            {takesRole &&
              (roleChoices.length > 1 ? (
                <OptionMenu
                  label="What they can do"
                  value={role ?? ""}
                  options={roleChoices.map((c) => ({ value: c.role, label: c.label }))}
                  disabled={!canManage || busy}
                  onChange={(next) =>
                    apply({
                      role: next,
                      noDownload: roles.find((r) => r.role === next)?.downloadOptional
                        ? value.noDownload
                        : false,
                    })
                  }
                />
              ) : (
                <span className="text-sm text-muted-foreground">
                  {roleChoices[0].label}
                </span>
              ))}
            {hasSettings && (
              <Popover>
                <PopoverTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Access settings"
                      disabled={!canManage || busy}
                    />
                  }
                >
                  <Settings2 />
                </PopoverTrigger>
                <PopoverContent align="start">
                  <div className="flex flex-col gap-4">
                    {downloadOptional && (
                      <div className="flex items-center gap-2">
                        <Checkbox
                          id="general-download"
                          checked={!value.noDownload}
                          onCheckedChange={(checked) =>
                            apply({ noDownload: !checked })
                          }
                        />
                        <Label htmlFor="general-download">Allow downloading</Label>
                      </div>
                    )}
                    {current.supportsExpiry && (
                      <div className="flex flex-col gap-2">
                        <Label htmlFor="general-expiry">Access expires</Label>
                        <Input
                          id="general-expiry"
                          type="date"
                          min={tomorrow}
                          value={isoToDateInput(value.expiresAt)}
                          onChange={(e) =>
                            apply({ expiresAt: endOfDayIso(e.target.value) })
                          }
                        />
                      </div>
                    )}
                  </div>
                </PopoverContent>
              </Popover>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
