import { useQuery } from "@apollo/client/react"
import { graphql } from "@/graphql"
import type { Permission } from "@/graphql/graphql"

export const GET_ME = graphql(`
  query GetMe {
    me {
      userId
      tenantId
      email
      displayName
      role
      permissions
      assignableRoles
    }
  }
`)

/**
 * What the signed-in user may do, as decided by the server. Show or hide admin
 * features with `can(...)`, never by comparing role names: what a role carries
 * is the server's business, and it can change without the UI changing.
 */
export function usePermissions() {
  const { data, loading } = useQuery(GET_ME)
  const me = data?.me
  const granted = new Set<Permission>(me?.permissions ?? [])

  return {
    me,
    loading,
    /** Whether the user holds the permission (false while still loading). */
    can: (permission: Permission) => granted.has(permission),
    /** The roles this user may give to others. */
    assignableRoles: me?.assignableRoles ?? [],
  }
}
