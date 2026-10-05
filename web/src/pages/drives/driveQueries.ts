import { graphql } from "@/graphql"

export const GET_DRIVES = graphql(`
  query GetDrives {
    drives {
      id
      name
      driveMetadata {
        driveType
      }
    }
  }
`)

export const CAN_CREATE_SHARED_DRIVE = graphql(`
  query CanCreateSharedDrive {
    canCreateSharedDrive
  }
`)

export const CREATE_SHARED_DRIVE = graphql(`
  mutation CreateSharedDrive($name: String!) {
    createSharedDrive(name: $name) {
      id
      name
    }
  }
`)
