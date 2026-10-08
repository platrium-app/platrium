import { Lock, MoreHorizontal, Network, Pencil, Power, Trash2 } from "lucide-react"
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
import { cn } from "@/lib/utils"
import { connectionFor } from "./connections"
import type { IdentityProviderNode } from "./types"

export type ProviderAction = "edit" | "disable" | "enable" | "delete"

function RowMenu({
  provider,
  onAction,
}: {
  provider: IdentityProviderNode
  onAction: (action: ProviderAction, provider: IdentityProviderNode) => void
}) {
  const hasUsers = provider.userCount > 0
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            className="size-7 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 data-[popup-open]:opacity-100"
            aria-label={`Actions for ${provider.name}`}
          />
        }
      >
        <MoreHorizontal className="size-4 text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48">
        <DropdownMenuItem onClick={() => onAction("edit", provider)}>
          <Pencil />
          Edit
        </DropdownMenuItem>
        {provider.enabled ? (
          <DropdownMenuItem onClick={() => onAction("disable", provider)}>
            <Power />
            Disable
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onClick={() => onAction("enable", provider)}>
            <Power />
            Enable
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        {hasUsers ? (
          <DropdownMenuGroup>
            <DropdownMenuLabel className="max-w-64 font-normal whitespace-normal text-muted-foreground">
              Can't be deleted while people sign in through it. Disable it
              instead.
            </DropdownMenuLabel>
          </DropdownMenuGroup>
        ) : (
          <DropdownMenuItem
            variant="destructive"
            onClick={() => onAction("delete", provider)}
          >
            <Trash2 />
            Delete
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function signInPolicy(p: IdentityProviderNode) {
  const who = p.jitUsers ? "Creates accounts" : "Existing accounts only"
  return p.allowedEmailDomains.length
    ? `${who} · ${p.allowedEmailDomains.join(", ")}`
    : who
}

/** The organization's identity providers. The built-in one is listed but never editable. */
export function IdentityProvidersTable({
  providers,
  onAction,
}: {
  providers: IdentityProviderNode[]
  onAction: (action: ProviderAction, provider: IdentityProviderNode) => void
}) {
  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
      <div className="grid grid-cols-12 gap-2 border-b px-4 py-3 text-sm font-medium text-muted-foreground select-none">
        <div className="col-span-8 sm:col-span-4">Provider</div>
        <div className="col-span-4 hidden sm:block">Sign-in</div>
        <div className="col-span-1 hidden md:block">Users</div>
        <div className="col-span-2 hidden md:block">Status</div>
        <div className="col-span-1" />
      </div>
      <div className="w-full flex-1 overflow-auto">
        {providers.map((p) => (
          <div
            key={p.id}
            className={cn(
              "group grid grid-cols-12 items-center gap-2 border-b px-4 py-3 text-sm text-foreground/90 transition-colors hover:bg-muted/50",
              !p.enabled && "text-muted-foreground"
            )}
          >
            <div className="col-span-8 flex min-w-0 items-center gap-2 sm:col-span-4">
              <div className="min-w-0">
                <div className="truncate font-medium">{p.name}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {p.isLocal
                    ? "Email and password, managed by Platrium"
                    : (connectionFor(p.type)?.label ?? p.type)}
                </div>
              </div>
            </div>
            <div className="col-span-4 hidden min-w-0 truncate text-xs text-muted-foreground sm:block">
              {p.isLocal ? "—" : signInPolicy(p)}
            </div>
            <div className="col-span-1 hidden md:block">{p.userCount}</div>
            <div className="col-span-2 hidden md:block">
              {p.isLocal ? (
                <Badge variant="outline">
                  <Lock />
                  Built-in
                </Badge>
              ) : p.enabled ? (
                <Badge variant="secondary">
                  <Network />
                  Enabled
                </Badge>
              ) : (
                <Badge variant="destructive">Disabled</Badge>
              )}
            </div>
            <div className="col-span-4 flex items-center justify-end sm:col-span-1">
              {!p.isLocal && <RowMenu provider={p} onAction={onAction} />}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
