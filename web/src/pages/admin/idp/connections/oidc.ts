import { UPDATE_OIDC_CONFIG } from "../idpQueries"
import type { ConnectionType } from "./index"
import { OidcFields } from "./OidcFields"

export interface OidcState {
  issuer: string
  clientId: string
  clientSecret: string
  /** Extra scopes, separated by spaces or commas. `openid` is always requested. */
  scopes: string
  emailClaim: string
  nameClaim: string
  pictureClaim: string
  requireEmailVerified: boolean
  extraParams: { name: string; value: string }[]
}

const splitList = (s: string) => s.split(/[\s,]+/).filter(Boolean)
const params = (s: OidcState) =>
  s.extraParams
    .map((p) => ({ name: p.name.trim(), value: p.value }))
    .filter((p) => p.name)

export const oidcConnection: ConnectionType<OidcState> = {
  type: "OIDC",
  label: "OpenID Connect",
  description:
    "Okta, Auth0, Entra ID, Google, Keycloak, Authentik and any other OpenID Connect provider.",
  initial: () => ({
    issuer: "",
    clientId: "",
    clientSecret: "",
    scopes: "email profile",
    emailClaim: "",
    nameClaim: "",
    pictureClaim: "",
    requireEmailVerified: true,
    extraParams: [],
  }),
  fromProvider: (p) => {
    const c = p.config?.__typename === "OidcConfig" ? p.config : null
    return {
      issuer: c?.issuer ?? "",
      clientId: c?.clientId ?? "",
      clientSecret: "",
      scopes: (c?.scopes ?? []).join(" "),
      emailClaim: c?.emailClaim ?? "",
      nameClaim: c?.nameClaim ?? "",
      pictureClaim: c?.pictureClaim ?? "",
      requireEmailVerified: c?.requireEmailVerified ?? true,
      extraParams: (c?.extraAuthParams ?? []).map((a) => ({ ...a })),
    }
  },
  Fields: OidcFields,
  createConfig: (s) => ({
    oidc: {
      issuer: s.issuer.trim(),
      clientId: s.clientId.trim(),
      clientSecret: s.clientSecret,
      scopes: splitList(s.scopes),
      emailClaim: s.emailClaim.trim() || undefined,
      nameClaim: s.nameClaim.trim() || undefined,
      pictureClaim: s.pictureClaim.trim() || undefined,
      requireEmailVerified: s.requireEmailVerified,
      extraAuthParams: params(s),
    },
  }),
  update: async (client, id, s) => {
    await client.mutate({
      mutation: UPDATE_OIDC_CONFIG,
      variables: {
        id,
        input: {
          clientId: s.clientId.trim(),
          // Empty keeps the stored secret.
          clientSecret: s.clientSecret || undefined,
          scopes: splitList(s.scopes),
          emailClaim: s.emailClaim.trim(),
          nameClaim: s.nameClaim.trim(),
          pictureClaim: s.pictureClaim.trim(),
          requireEmailVerified: s.requireEmailVerified,
          extraAuthParams: params(s),
        },
      },
    })
  },
}
