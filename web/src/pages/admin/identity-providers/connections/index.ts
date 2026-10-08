import type { ComponentType } from "react"
import type { ApolloClient } from "@apollo/client"
import type { ProviderConfigInput } from "@/graphql/graphql"
import type { IdentityProviderNode } from "../types"
import { oidcConnection } from "./oidc"

export type ConnectionMode = "create" | "edit"

export interface ConnectionFieldsProps<S> {
  state: S
  onChange: (next: S) => void
  mode: ConnectionMode
  disabled: boolean
  /** The address to register with the provider; known once it exists. */
  redirectUri?: string
}

/**
 * Everything the admin console knows about one kind of identity provider. The
 * settings every provider shares (name, how users are created) are rendered
 * once by the dialog; a type only contributes its connection settings. A new
 * protocol (SAML in the enterprise edition) is one more entry in
 * CONNECTION_TYPES and nothing else changes.
 */
export interface ConnectionType<S> {
  /** Matches `IdentityProvider.type`. */
  type: string
  label: string
  description: string
  initial: () => S
  fromProvider: (provider: IdentityProviderNode) => S
  Fields: ComponentType<ConnectionFieldsProps<S>>
  /** The `config` of a create request. */
  createConfig: (state: S) => ProviderConfigInput
  /** Saves changed connection settings of an existing provider. */
  update: (client: ApolloClient, id: string, state: S) => Promise<void>
}

// The list is heterogeneous (each entry has its own state), hence `any`: the
// dialog only ever hands an entry back its own state.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const CONNECTION_TYPES: ConnectionType<any>[] = [oidcConnection]

export function connectionFor(type: string) {
  return CONNECTION_TYPES.find((c) => c.type === type)
}
