import { useState } from "react"
import { useMutation } from "@apollo/client/react"
import { ChevronDown, Plus, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage } from "@/lib/errors"
import { TEST_OIDC_DISCOVERY } from "../idpQueries"
import { RedirectUriBox } from "../RedirectUriBox"
import type { ConnectionFieldsProps } from "./index"
import type { OidcState } from "./oidc"

export function OidcFields({
  state,
  onChange,
  mode,
  disabled,
  redirectUri,
}: ConnectionFieldsProps<OidcState>) {
  const set = (patch: Partial<OidcState>) => onChange({ ...state, ...patch })
  const [testDiscovery, { data, loading: testing, error }] = useMutation(
    TEST_OIDC_DISCOVERY
  )
  const [advanced, setAdvanced] = useState(false)
  const result = data?.testOidcDiscovery
  const editing = mode === "edit"

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="oidc-issuer">Issuer URL</Label>
        <div className="flex gap-2">
          <Input
            id="oidc-issuer"
            value={state.issuer}
            onChange={(e) => set({ issuer: e.target.value })}
            placeholder="https://your-tenant.auth0.com/"
            autoComplete="off"
            spellCheck={false}
            required
            readOnly={editing}
            disabled={disabled}
          />
          <Button
            type="button"
            variant="outline"
            disabled={disabled || testing || !state.issuer.trim()}
            onClick={() =>
              testDiscovery({ variables: { issuer: state.issuer } }).catch(
                () => {}
              )
            }
          >
            {testing && <Spinner className="mr-2 size-4" />}
            Test
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          {editing
            ? "The issuer can't be changed: users are known to the provider that issued their identity. Add a new provider instead."
            : "The address your provider publishes, e.g. https://acme.auth0.com. A trailing slash doesn't matter."}
        </p>
        {error && (
          <p className="text-sm font-medium text-destructive">
            {errorMessage(error)}
          </p>
        )}
        {result && !result.ok && (
          <p className="text-sm font-medium text-destructive">
            {result.message}
          </p>
        )}
        {result?.ok && (
          <p className="text-sm text-emerald-600 dark:text-emerald-400">
            Found an OpenID Connect provider.
            {result.supportsPkce === false &&
              " It doesn't advertise PKCE (S256) support, which Platrium requires."}
          </p>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="oidc-client-id">Client ID</Label>
          <Input
            id="oidc-client-id"
            value={state.clientId}
            onChange={(e) => set({ clientId: e.target.value })}
            autoComplete="off"
            required
            disabled={disabled}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="oidc-client-secret">Client secret</Label>
          <Input
            id="oidc-client-secret"
            type="password"
            value={state.clientSecret}
            onChange={(e) => set({ clientSecret: e.target.value })}
            placeholder={editing ? "Unchanged" : undefined}
            autoComplete="new-password"
            required={!editing}
            disabled={disabled}
          />
        </div>
      </div>
      {editing && (
        <p className="-mt-2 text-xs text-muted-foreground">
          Leave the client secret empty to keep the current one.
        </p>
      )}

      {redirectUri && <RedirectUriBox uri={redirectUri} />}

      <Collapsible open={advanced} onOpenChange={setAdvanced}>
        <CollapsibleTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="-ml-2 text-muted-foreground"
            />
          }
        >
          <ChevronDown
            className={`size-4 transition-transform ${advanced ? "" : "-rotate-90"}`}
          />
          Advanced
        </CollapsibleTrigger>
        <CollapsibleContent className="flex flex-col gap-4 pt-3">
          <div className="flex flex-col gap-2">
            <Label htmlFor="oidc-scopes">Additional scopes</Label>
            <Input
              id="oidc-scopes"
              value={state.scopes}
              onChange={(e) => set({ scopes: e.target.value })}
              placeholder="email profile"
              disabled={disabled}
            />
            <p className="text-xs text-muted-foreground">
              <code>openid</code> is always requested. Most providers need{" "}
              <code>email</code> and <code>profile</code> for the details
              below.
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-3">
            {(
              [
                ["emailClaim", "Email claim", "email"],
                ["nameClaim", "Name claim", "name"],
                ["pictureClaim", "Picture claim", "picture"],
              ] as const
            ).map(([key, label, placeholder]) => (
              <div key={key} className="flex flex-col gap-2">
                <Label htmlFor={`oidc-${key}`}>{label}</Label>
                <Input
                  id={`oidc-${key}`}
                  value={state[key]}
                  onChange={(e) => set({ [key]: e.target.value })}
                  placeholder={placeholder}
                  disabled={disabled}
                />
              </div>
            ))}
          </div>

          <div className="flex items-center gap-2">
            <Checkbox
              id="oidc-verified"
              checked={state.requireEmailVerified}
              onCheckedChange={(v) => set({ requireEmailVerified: v === true })}
              disabled={disabled}
            />
            <Label htmlFor="oidc-verified" className="font-normal">
              Require the provider to have verified the user's email
            </Label>
          </div>

          <div className="flex flex-col gap-2">
            <Label>Extra authorization parameters</Label>
            {state.extraParams.map((p, i) => (
              <div key={i} className="flex gap-2">
                <Input
                  aria-label="Parameter name"
                  placeholder="audience"
                  value={p.name}
                  onChange={(e) =>
                    set({
                      extraParams: state.extraParams.map((q, j) =>
                        j === i ? { ...q, name: e.target.value } : q
                      ),
                    })
                  }
                  disabled={disabled}
                />
                <Input
                  aria-label="Parameter value"
                  value={p.value}
                  onChange={(e) =>
                    set({
                      extraParams: state.extraParams.map((q, j) =>
                        j === i ? { ...q, value: e.target.value } : q
                      ),
                    })
                  }
                  disabled={disabled}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Remove parameter"
                  onClick={() =>
                    set({
                      extraParams: state.extraParams.filter((_, j) => j !== i),
                    })
                  }
                  disabled={disabled}
                >
                  <X className="size-4" />
                </Button>
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="w-fit"
              onClick={() =>
                set({
                  extraParams: [...state.extraParams, { name: "", value: "" }],
                })
              }
              disabled={disabled}
            >
              <Plus className="size-4" />
              Add parameter
            </Button>
            <p className="text-xs text-muted-foreground">
              Sent on the sign-in request, for providers that need more than
              the standard ones (Auth0's <code>audience</code>, for example).
            </p>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </div>
  )
}
