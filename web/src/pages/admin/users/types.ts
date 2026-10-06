import type { ResultOf } from "@graphql-typed-document-node/core"
import type { ADMIN_USERS } from "./adminUserQueries"

export type AdminUserNode = NonNullable<
  ResultOf<typeof ADMIN_USERS>["adminUsers"]
>["edges"][number]["node"]
