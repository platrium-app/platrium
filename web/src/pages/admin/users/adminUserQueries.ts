import { graphql } from "@/graphql"

export const ADMIN_USERS = graphql(`
  query AdminUsers(
    $first: Int
    $after: String
    $search: String
    $sourceId: ID
    $status: UserStatus
  ) {
    adminUsers(
      first: $first
      after: $after
      search: $search
      sourceId: $sourceId
      status: $status
    ) {
      totalCount
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        cursor
        node {
          id
          email
          displayName
          role
          disabled
          disabledAt
          createdAt
          editable
          manageable
          source {
            id
            name
            type
            isLocal
          }
        }
      }
    }
  }
`)

export const ADMIN_IDENTITY_SOURCES = graphql(`
  query AdminIdentitySources {
    adminIdentitySources {
      id
      name
      type
      isLocal
    }
  }
`)

export const CREATE_LOCAL_USER = graphql(`
  mutation CreateLocalUser($input: CreateLocalUserInput!) {
    createLocalUser(input: $input) {
      id
    }
  }
`)

export const UPDATE_LOCAL_USER = graphql(`
  mutation UpdateLocalUser($id: ID!, $input: UpdateLocalUserInput!) {
    updateLocalUser(id: $id, input: $input) {
      id
      displayName
      role
      manageable
    }
  }
`)

export const RESET_LOCAL_USER_PASSWORD = graphql(`
  mutation ResetLocalUserPassword($id: ID!, $password: String!) {
    resetLocalUserPassword(id: $id, password: $password)
  }
`)

export const SET_USER_DISABLED = graphql(`
  mutation SetUserDisabled($id: ID!, $disabled: Boolean!) {
    setUserDisabled(id: $id, disabled: $disabled) {
      id
      disabled
      disabledAt
    }
  }
`)
