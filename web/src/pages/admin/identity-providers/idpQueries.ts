import { graphql } from "@/graphql"

export const IDENTITY_PROVIDERS = graphql(`
  query IdentityProviders {
    identityProviders {
      id
      name
      type
      isLocal
      enabled
      jitUsers
      defaultRole
      allowedEmailDomains
      userCount
      config {
        __typename
        ... on OidcConfig {
          issuer
          clientId
          scopes
          emailClaim
          nameClaim
          pictureClaim
          requireEmailVerified
          extraAuthParams {
            name
            value
          }
          redirectUri
        }
      }
    }
  }
`)

export const CREATE_IDENTITY_PROVIDER = graphql(`
  mutation CreateIdentityProvider($input: CreateIdentityProviderInput!) {
    createIdentityProvider(input: $input) {
      id
      config {
        __typename
        ... on OidcConfig {
          redirectUri
        }
      }
    }
  }
`)

export const UPDATE_IDENTITY_PROVIDER = graphql(`
  mutation UpdateIdentityProvider($id: ID!, $input: UpdateIdentityProviderInput!) {
    updateIdentityProvider(id: $id, input: $input) {
      id
    }
  }
`)

export const UPDATE_OIDC_CONFIG = graphql(`
  mutation UpdateOidcConfig($id: ID!, $input: UpdateOidcConfigInput!) {
    updateOidcConfig(id: $id, input: $input) {
      id
    }
  }
`)

export const SET_IDENTITY_PROVIDER_ENABLED = graphql(`
  mutation SetIdentityProviderEnabled($id: ID!, $enabled: Boolean!) {
    setIdentityProviderEnabled(id: $id, enabled: $enabled) {
      id
      enabled
    }
  }
`)

export const DELETE_IDENTITY_PROVIDER = graphql(`
  mutation DeleteIdentityProvider($id: ID!) {
    deleteIdentityProvider(id: $id)
  }
`)

export const TEST_OIDC_DISCOVERY = graphql(`
  mutation TestOidcDiscovery($issuer: String!) {
    testOidcDiscovery(issuer: $issuer) {
      ok
      message
      authorizationEndpoint
      tokenEndpoint
      scopesSupported
      supportsPkce
    }
  }
`)
