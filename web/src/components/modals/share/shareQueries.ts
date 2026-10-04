import { graphql } from "@/graphql"

export const ITEM_ACCESS = graphql(`
  query ItemAccess($itemId: ID!) {
    itemAccess(itemId: $itemId) {
      itemId
      inheritsPermissions
      owner {
        type
        id
        name
        email
      }
      generalAccess {
        level
        role
        noDownload
        expiresAt
      }
      grants {
        id
        subjectType
        subjectId
        subjectName
        role
        noDownload
        capabilities
        expiresAt
      }
    }
  }
`)

export const SHARE_ROLES = graphql(`
  query ShareRoles($itemId: ID!) {
    shareRoles(itemId: $itemId) {
      role
      label
      description
      capabilities
    }
  }
`)

export const SEARCH_DIRECTORY = graphql(`
  query SearchDirectory($query: String!, $first: Int, $exclude: ID) {
    searchDirectory(
      query: $query
      first: $first
      excludeAccessToItemId: $exclude
    ) {
      type
      id
      name
      email
    }
  }
`)

export const SHARE_ITEM = graphql(`
  mutation ShareItem($input: ShareInput!) {
    shareItem(input: $input) {
      id
      subjectType
      subjectId
      subjectName
      role
      noDownload
      capabilities
      expiresAt
    }
  }
`)

export const REVOKE_ACCESS = graphql(`
  mutation RevokeAccess($grantId: ID!) {
    revokeAccess(grantId: $grantId)
  }
`)

export const SET_GENERAL_ACCESS = graphql(`
  mutation SetGeneralAccess($input: GeneralAccessInput!) {
    setGeneralAccess(input: $input) {
      level
      role
      noDownload
      expiresAt
    }
  }
`)

export const SET_INHERITANCE = graphql(`
  mutation SetInheritance($itemId: ID!, $inherit: Boolean!) {
    setInheritance(itemId: $itemId, inherit: $inherit)
  }
`)
