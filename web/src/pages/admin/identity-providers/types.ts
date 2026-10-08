import type { ResultOf } from "@graphql-typed-document-node/core"
import type { IDENTITY_PROVIDERS } from "./idpQueries"

export type IdentityProviderNode = ResultOf<
  typeof IDENTITY_PROVIDERS
>["identityProviders"][number]
