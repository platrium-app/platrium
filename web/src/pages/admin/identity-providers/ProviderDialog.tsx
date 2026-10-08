import { useState, type FormEvent } from "react"
import { useApolloClient, useMutation } from "@apollo/client/react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { NativeSelect } from "@/components/ui/native-select"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage } from "@/lib/errors"
import { CONNECTION_TYPES, connectionFor } from "./connections"
import {
  CREATE_IDENTITY_PROVIDER,
  UPDATE_IDENTITY_PROVIDER,
} from "./idpQueries"
import { RedirectUriBox } from "./RedirectUriBox"
import type { IdentityProviderNode } from "./types"

// People signing in through a provider start as ordinary members. Giving
// someone more is done afterwards, by an administrator, in Users.
const DEFAULT_ROLE = "MEMBER"

const parseDomains = (s: string) =>
  s
    .split(/[\s,]+/)
    .map((d) => d.replace(/^@/, "").trim())
    .filter(Boolean)

/**
 * Creates a provider, or edits one. The name and how users are created are the
 * same for every provider type; the connection settings come from the type's
 * entry in CONNECTION_TYPES.
 */
export function ProviderDialog({
  provider,
  onClose,
}: {
  /** The provider to edit; omitted to create one. */
  provider?: IdentityProviderNode
  onClose: () => void
}) {
  const client = useApolloClient()
  const refetch = { refetchQueries: ["IdentityProviders"], awaitRefetchQueries: true }
  const [create] = useMutation(CREATE_IDENTITY_PROVIDER, refetch)
  const [update] = useMutation(UPDATE_IDENTITY_PROVIDER, refetch)

  const editing = Boolean(provider)
  const [type, setType] = useState(provider?.type ?? CONNECTION_TYPES[0].type)
  const connection = connectionFor(type)

  const [name, setName] = useState(provider?.name ?? "")
  const [jitUsers, setJitUsers] = useState(provider?.jitUsers ?? true)
  const [domains, setDomains] = useState(
    (provider?.allowedEmailDomains ?? []).join(", ")
  )
  const [state, setState] = useState(() =>
    provider ? connectionFor(provider.type)?.fromProvider(provider) : connection?.initial()
  )
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [createdUri, setCreatedUri] = useState<string | null>(null)

  const redirectUri =
    provider?.config?.__typename === "OidcConfig"
      ? provider.config.redirectUri
      : undefined

  if (!connection) return null

  const changeType = (next: string) => {
    setType(next)
    setState(connectionFor(next)?.initial())
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      if (provider) {
        await update({
          variables: {
            id: provider.id,
            input: { name, jitUsers, allowedEmailDomains: parseDomains(domains) },
          },
        })
        await connection.update(client, provider.id, state)
        await client.refetchQueries({ include: ["IdentityProviders"] })
        onClose()
      } else {
        const { data } = await create({
          variables: {
            input: {
              name,
              jitUsers,
              defaultRole: DEFAULT_ROLE,
              allowedEmailDomains: parseDomains(domains),
              config: connection.createConfig(state),
            },
          },
        })
        const cfg = data?.createIdentityProvider.config
        // The redirect URL needs the provider's ID, so it only exists now.
        setCreatedUri(cfg?.__typename === "OidcConfig" ? cfg.redirectUri : "")
      }
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  if (createdUri !== null) {
    return (
      <Dialog open onOpenChange={(open) => !open && onClose()}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Provider added</DialogTitle>
            <DialogDescription>
              One last step: tell {name} where to send people back to after
              they sign in.
            </DialogDescription>
          </DialogHeader>
          {createdUri && <RedirectUriBox uri={createdUri} />}
          <DialogFooter>
            <Button onClick={onClose}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {editing ? `Edit ${provider?.name}` : "Add identity provider"}
          </DialogTitle>
          <DialogDescription>
            {editing
              ? connection.label
              : "Let people sign in with an account they already have."}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="flex flex-col gap-5">
          {!editing && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="idp-type">Provider type</Label>
              <NativeSelect
                id="idp-type"
                value={type}
                onChange={(e) => changeType(e.target.value)}
                disabled={busy}
              >
                {CONNECTION_TYPES.map((c) => (
                  <option key={c.type} value={c.type}>
                    {c.label}
                  </option>
                ))}
              </NativeSelect>
              <p className="text-xs text-muted-foreground">
                {connection.description}
              </p>
            </div>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="idp-name">Name</Label>
            <Input
              id="idp-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Acme single sign-on"
              autoFocus
              required
              disabled={busy}
            />
            <p className="text-xs text-muted-foreground">
              Shown on the sign-in page, as "Login with {name || "…"}".
            </p>
          </div>

          <connection.Fields
            state={state}
            onChange={setState}
            mode={editing ? "edit" : "create"}
            disabled={busy}
            redirectUri={redirectUri}
          />

          <div className="flex flex-col gap-3 rounded-lg border p-3">
            <div className="flex items-start gap-2">
              <Checkbox
                id="idp-jit"
                checked={jitUsers}
                onCheckedChange={(v) => setJitUsers(v === true)}
                disabled={busy}
                className="mt-0.5"
              />
              <div className="flex flex-col gap-1">
                <Label htmlFor="idp-jit">Create accounts on first sign-in</Label>
                <p className="text-xs text-muted-foreground">
                  {jitUsers
                    ? "Anyone who signs in with this provider gets an account, as a member."
                    : "Only people who already have an account can sign in."}
                </p>
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="idp-domains">Allowed email domains</Label>
              <Input
                id="idp-domains"
                value={domains}
                onChange={(e) => setDomains(e.target.value)}
                placeholder="acme.com, acme.org"
                disabled={busy}
              />
              <p className="text-xs text-muted-foreground">
                Leave empty to allow any address the provider vouches for.
              </p>
            </div>
          </div>

          {error && (
            <p className="text-sm font-medium text-destructive">{error}</p>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={busy}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Spinner className="mr-2 size-4" />}
              {editing ? "Save changes" : "Add provider"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
