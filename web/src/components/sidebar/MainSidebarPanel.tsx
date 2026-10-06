import * as React from "react"
import { useLocation, useNavigate } from "react-router-dom"
import { useQuery, useApolloClient } from "@apollo/client/react"
import { graphql } from "@/graphql"
import {
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import {
  ChevronRight,
  ChevronDown,
  FolderRoot,
  BookUser,
  CircleAlertIcon,
} from "lucide-react"
import { FilesystemTree } from "@/components/custom/FilesystemTreeView"
import { Spinner } from "@/components/ui/spinner"
import { usePermissions } from "@/hooks/usePermissions"
import { GET_DRIVES } from "@/pages/drives/driveQueries"
import { NavSection } from "./NavSection"
import { ADMIN_CONSOLE_ENTRY, MAIN_NAV_ITEMS, LOCATIONS_NAV_ITEMS } from "./nav-config"

export type FolderNode = {
  id: string
  parentId: string | null
  name: string
  hasChildren: boolean
  icon?: React.ElementType
  isLoading?: boolean
}

/* Graph QL Queries */
const GET_SUBFOLDERS = graphql(`
  query GetSubfoldersSidebar($folderId: ID!) {
    folderContents(folderId: $folderId, first: 100) {
      edges {
        node {
          id
          name
          type
        }
      }
    }
  }
`)

type FolderContentsResult = {
  folderContents?: {
    edges?:
    { node?: { id: string; name: string; type: string } | null }[] | null
  } | null
}

const SHARED_DRIVES_PATH = "/shared-drives"

/** The everyday sidebar: navigation, and the drives tree. */
export function MainSidebarPanel() {
  const navigate = useNavigate()
  const location = useLocation()
  const { can } = usePermissions()

  const [expandedSharedDrives, setExpandedSharedDrives] = React.useState(true)

  const { data, loading, error } = useQuery(GET_DRIVES)

  const privateDrives: FolderNode[] = []
  const sharedDrives: FolderNode[] = []

  if (data && data.drives) {
    data.drives.forEach((drive) => {
      const node: FolderNode = {
        id: drive.id,
        parentId: null,
        name: drive.name,
        hasChildren: true,
        icon: FolderRoot,
      }
      const driveType = drive.driveMetadata?.driveType
      if (driveType === "PRIVATE") {
        privateDrives.push(node)
      } else if (driveType === "SHARED") {
        sharedDrives.push(node)
      }
    })
  }

  const apolloClient = useApolloClient()

  // Helper function for the generic tree
  const getChildren = React.useCallback(
    async (parentId: string) => {
      try {
        const result = await apolloClient.query({
          query: GET_SUBFOLDERS,
          variables: { folderId: parentId },
        })

        const children: FolderNode[] = []
          ; (result.data as FolderContentsResult).folderContents?.edges?.forEach(
            (edge) => {
              if (edge?.node && edge.node.type === "FOLDER") {
                children.push({
                  id: edge.node.id,
                  parentId,
                  name: edge.node.name,
                  hasChildren: true, // We don't know, so assume true to show chevron
                })
              }
            }
          )
        return children
      } catch (err) {
        console.error("Failed to fetch subfolders", err)
        return []
      }
    },
    [apolloClient]
  )

  return (
    <SidebarContent>
      {/* Top Static Links */}
      <SidebarGroup>
        <SidebarGroupContent>
          <NavSection items={MAIN_NAV_ITEMS} />
        </SidebarGroupContent>
      </SidebarGroup>

      {/* Locations */}
      <SidebarGroup>
        <SidebarGroupLabel>Locations</SidebarGroupLabel>
        <SidebarGroupContent>
          <NavSection items={LOCATIONS_NAV_ITEMS} />
        </SidebarGroupContent>
      </SidebarGroup>

      {/* Unified Drives Group */}
      <SidebarGroup className="min-h-0 flex-1 overflow-hidden">
        <SidebarGroupLabel>Drives</SidebarGroupLabel>
        <SidebarGroupContent className="no-scrollbar min-h-0 flex-1 overflow-y-auto">
          <SidebarMenu className="group/tree">
            {/* 1. My Drive */}
            {loading ? (
              <div className="flex items-center gap-2 px-3 text-sm text-muted-foreground">
                <Spinner />
                Loading drives
              </div>
            ) : error ? (
              <div className="flex items-center gap-2 px-3 text-sm text-destructive">
                <CircleAlertIcon className="size-4" />
                Failed to Load Drives
              </div>
            ) : (
              <FilesystemTree
                nodes={privateDrives}
                getChildren={getChildren}
                activeId={location.pathname.split("/").pop()}
                onSelect={(id) => navigate(`/folder/${id}`)}
              />
            )}

            {/* 2. Shared Drives: always shown; the label opens the list, the chevron expands it */}
            <SidebarMenuItem>
              <SidebarMenuButton
                isActive={location.pathname === SHARED_DRIVES_PATH}
                onClick={() => {
                  setExpandedSharedDrives(true)
                  navigate(SHARED_DRIVES_PATH)
                }}
                style={{ paddingLeft: "8px" }}
              >
                <span
                  role="button"
                  aria-label={
                    expandedSharedDrives
                      ? "Collapse shared drives"
                      : "Expand shared drives"
                  }
                  className="flex-shrink-0 cursor-pointer"
                  onClick={(e) => {
                    e.stopPropagation()
                    setExpandedSharedDrives((prev) => !prev)
                  }}
                >
                  {expandedSharedDrives ? (
                    <ChevronDown className="size-4" />
                  ) : (
                    <ChevronRight className="size-4" />
                  )}
                </span>
                <BookUser className="size-4 flex-shrink-0" />
                <span className="truncate">Shared Drives</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            {expandedSharedDrives && sharedDrives.length > 0 && (
              <ul className="relative flex w-full min-w-0 flex-col gap-0.5">
                <div
                  className="pointer-events-none absolute top-0 bottom-0 z-10 w-px bg-sidebar-border opacity-0 transition-opacity duration-200 group-hover/tree:opacity-100"
                  style={{ left: "16px" }}
                />
                <FilesystemTree
                  nodes={sharedDrives}
                  getChildren={getChildren}
                  activeId={location.pathname.split("/").pop()}
                  onSelect={(id) => navigate(`/folder/${id}`)}
                  level={1}
                />
              </ul>
            )}
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>

    </SidebarContent>
  )
}
