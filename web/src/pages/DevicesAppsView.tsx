import { useCallback, useEffect, useState } from "react"
import { Check, Copy, Laptop, Plus, Smartphone, Terminal, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useSetBreadcrumbs } from "@/contexts/BreadcrumbContext"

interface ClientInfo {
  id: string
  kind: "DEVICE" | "APP"
  name: string
  platform?: string
  app_version?: string
  created_at: string
  last_used_at: string
  expires_at?: string
  current: boolean
}

const EXPIRY_OPTIONS = [
  { label: "30 days", days: 30 },
  { label: "90 days", days: 90 },
  { label: "1 year", days: 365 },
  { label: "No expiry (until unused)", days: 0 },
]

function formatWhen(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })
}

function ClientIcon({ client }: { client: ClientInfo }) {
  if (client.kind === "APP") return <Terminal className="size-5" />
  return client.platform === "MACOS" ? <Laptop className="size-5" /> : <Smartphone className="size-5" />
}

function ClientRow({ client, onRevoke }: { client: ClientInfo; onRevoke: (c: ClientInfo) => void }) {
  const details = [client.platform, client.app_version && `v${client.app_version}`]
    .filter(Boolean)
    .join(" · ")
  return (
    <li className="flex items-center gap-3 rounded-md border p-3">
      <ClientIcon client={client} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate font-medium">{client.name}</span>
          {client.current && (
            <span className="rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary">This session</span>
          )}
        </div>
        <div className="text-xs text-muted-foreground">
          {details && <>{details} · </>}Last used {formatWhen(client.last_used_at)}
          {client.expires_at && <> · Expires {formatWhen(client.expires_at)}</>}
        </div>
      </div>
      <Button variant="ghost" size="icon" aria-label={`Revoke ${client.name}`} onClick={() => onRevoke(client)}>
        <Trash2 className="size-4" />
      </Button>
    </li>
  )
}

/** Devices and apps signed in as the current user, with revoke and manual app tokens. */
export default function DevicesAppsView() {
  useSetBreadcrumbs([{ label: "Devices & Apps", icon: Smartphone }])

  const [clients, setClients] = useState<ClientInfo[] | null>(null)
  const [error, setError] = useState("")
  const [revoking, setRevoking] = useState<ClientInfo | null>(null)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState("")
  const [newDays, setNewDays] = useState(90)
  const [created, setCreated] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await fetch("/api/auth/clients")
      if (!res.ok) throw new Error()
      setClients((await res.json()).clients)
    } catch {
      setError("Could not load your devices and apps.")
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const revoke = async () => {
    if (!revoking) return
    setBusy(true)
    const res = await fetch(`/api/auth/clients/${encodeURIComponent(revoking.id)}`, { method: "DELETE" })
    setBusy(false)
    if (res.ok || res.status === 404) {
      setRevoking(null)
      load()
    } else {
      setError("Could not revoke. Please try again.")
    }
  }

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    const res = await fetch("/api/auth/clients", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: newName, expires_in_days: newDays || undefined }),
    })
    setBusy(false)
    if (!res.ok) {
      setError("Could not create the token.")
      return
    }
    setCreated((await res.json()).token)
    setNewName("")
    load()
  }

  const closeCreate = () => {
    setCreating(false)
    setCreated(null)
    setCopied(false)
  }

  const devices = clients?.filter((c) => c.kind === "DEVICE") ?? []
  const apps = clients?.filter((c) => c.kind === "APP") ?? []

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-8 overflow-auto p-2">
      {error && <div className="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>}
      {!clients && !error && <Spinner className="mx-auto size-6 text-muted-foreground" />}

      {clients && (
        <>
          <section className="flex flex-col gap-3">
            <h2 className="text-lg font-semibold">Devices</h2>
            <p className="text-sm text-muted-foreground">
              Phones and computers where you are signed in to a Platrium app.
            </p>
            {devices.length === 0 ? (
              <p className="text-sm text-muted-foreground">No devices yet.</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {devices.map((c) => <ClientRow key={c.id} client={c} onRevoke={setRevoking} />)}
              </ul>
            )}
          </section>

          <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold">Apps</h2>
              <Button size="sm" onClick={() => setCreating(true)}>
                <Plus className="size-4" /> Create token
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              Command-line tools, drive mounts and scripts that access Platrium as you.
            </p>
            {apps.length === 0 ? (
              <p className="text-sm text-muted-foreground">No apps yet.</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {apps.map((c) => <ClientRow key={c.id} client={c} onRevoke={setRevoking} />)}
              </ul>
            )}
          </section>
        </>
      )}

      <Dialog open={!!revoking} onOpenChange={(o) => !o && setRevoking(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revoke “{revoking?.name}”?</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            It will be signed out immediately and will need your approval to connect again.
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setRevoking(null)} disabled={busy}>Cancel</Button>
            <Button variant="destructive" onClick={revoke} disabled={busy}>
              {busy ? <Spinner /> : "Revoke"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={creating} onOpenChange={(o) => !o && closeCreate()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{created ? "Copy your token" : "Create an app token"}</DialogTitle>
          </DialogHeader>
          {created ? (
            <div className="flex flex-col gap-3">
              <p className="text-sm text-muted-foreground">
                This is the only time it will be shown. Store it somewhere safe.
              </p>
              <div className="flex items-center gap-2">
                <code className="min-w-0 flex-1 break-all rounded bg-muted p-2 text-xs">{created}</code>
                <Button
                  variant="outline"
                  size="icon"
                  aria-label="Copy token"
                  onClick={async () => {
                    await navigator.clipboard.writeText(created)
                    setCopied(true)
                  }}
                >
                  {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                </Button>
              </div>
              <DialogFooter>
                <Button onClick={closeCreate}>Done</Button>
              </DialogFooter>
            </div>
          ) : (
            <form onSubmit={create} className="flex flex-col gap-3">
              <Input
                placeholder="Name, e.g. CI pipeline"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                required
                autoFocus
              />
              <select
                className="h-9 rounded-md border bg-background px-3 text-sm"
                value={newDays}
                onChange={(e) => setNewDays(Number(e.target.value))}
              >
                {EXPIRY_OPTIONS.map((o) => (
                  <option key={o.days} value={o.days}>Expires: {o.label}</option>
                ))}
              </select>
              <DialogFooter>
                <Button type="button" variant="ghost" onClick={closeCreate}>Cancel</Button>
                <Button type="submit" disabled={busy || !newName.trim()}>
                  {busy ? <Spinner /> : "Create"}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
