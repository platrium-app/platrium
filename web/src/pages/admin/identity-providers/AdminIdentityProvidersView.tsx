import { useState } from "react"
import { useMutation, useQuery } from "@apollo/client/react"
import { CircleAlertIcon, KeyRound, Plus, ShieldCheck } from "lucide-react"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { ConfirmModal } from "@/components/modals/ConfirmModal"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { useSetBreadcrumbs } from "@/contexts/BreadcrumbContext"
import { errorMessage } from "@/lib/errors"
import {
  DELETE_IDENTITY_PROVIDER,
  IDENTITY_PROVIDERS,
  SET_IDENTITY_PROVIDER_ENABLED,
} from "./idpQueries"
import {
  IdentityProvidersTable,
  type ProviderAction,
} from "./IdentityProvidersTable"
import { ProviderDialog } from "./ProviderDialog"
import type { IdentityProviderNode } from "./types"

type OpenDialog =
  | { kind: "create" }
  | { kind: "edit"; provider: IdentityProviderNode }
  | { kind: "disable"; provider: IdentityProviderNode }
  | { kind: "delete"; provider: IdentityProviderNode }

/** Where the organization's people sign in: the built-in provider and any external ones. */
export default function AdminIdentityProvidersView() {
  useSetBreadcrumbs([
    { label: "Admin console", icon: ShieldCheck },
    { label: "Identity providers", icon: KeyRound },
  ])

  const { data, loading, error } = useQuery(IDENTITY_PROVIDERS)
  const [setEnabled] = useMutation(SET_IDENTITY_PROVIDER_ENABLED)
  const [remove] = useMutation(DELETE_IDENTITY_PROVIDER, {
    refetchQueries: ["IdentityProviders"],
    awaitRefetchQueries: true,
  })
  const [dialog, setDialog] = useState<OpenDialog | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const providers = data?.identityProviders ?? []

  const handleAction = async (
    action: ProviderAction,
    provider: IdentityProviderNode
  ) => {
    setActionError(null)
    if (action === "enable") {
      try {
        await setEnabled({ variables: { id: provider.id, enabled: true } })
      } catch (err) {
        setActionError(errorMessage(err))
      }
      return
    }
    setDialog({ kind: action, provider })
  }

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden">
      <div className="flex shrink-0 flex-wrap items-center gap-2 pb-3">
        <p className="max-w-xl text-sm text-muted-foreground">
          The accounts people can sign in with. The built-in provider always
          stays; add others so people can use the accounts they already have.
        </p>
        <div className="ml-auto">
          <Button onClick={() => setDialog({ kind: "create" })}>
            <Plus className="size-4" />
            <span>Add provider</span>
          </Button>
        </div>
      </div>

      {actionError && (
        <p className="shrink-0 pb-2 text-sm font-medium text-destructive">
          {actionError}
        </p>
      )}

      {loading && !data ? (
        <div className="flex flex-1 items-center justify-center">
          <Spinner className="size-6 text-muted-foreground" />
        </div>
      ) : error ? (
        <PlaceholderView
          icon={CircleAlertIcon}
          variant="error"
          title="Couldn't load identity providers"
          description={error.message}
        />
      ) : (
        <IdentityProvidersTable providers={providers} onAction={handleAction} />
      )}

      {dialog?.kind === "create" && (
        <ProviderDialog onClose={() => setDialog(null)} />
      )}
      {dialog?.kind === "edit" && (
        <ProviderDialog
          provider={dialog.provider}
          onClose={() => setDialog(null)}
        />
      )}
      <ConfirmModal
        isOpen={dialog?.kind === "disable"}
        onClose={() => setDialog(null)}
        title={`Disable ${dialog?.kind === "disable" ? dialog.provider.name : ""}?`}
        description="People can't sign in through it until it is enabled again. Their accounts and files stay, and anyone already signed in stays signed in until their session ends."
        confirmLabel="Disable"
        destructive
        onConfirm={() =>
          setEnabled({
            variables: {
              id: (dialog as { provider: IdentityProviderNode }).provider.id,
              enabled: false,
            },
          })
        }
      />
      <ConfirmModal
        isOpen={dialog?.kind === "delete"}
        onClose={() => setDialog(null)}
        title={`Delete ${dialog?.kind === "delete" ? dialog.provider.name : ""}?`}
        description="This removes the provider and its settings. It can't be undone."
        confirmLabel="Delete"
        destructive
        onConfirm={() =>
          remove({
            variables: {
              id: (dialog as { provider: IdentityProviderNode }).provider.id,
            },
          })
        }
      />
    </div>
  )
}
