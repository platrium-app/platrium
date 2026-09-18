"use client"

import * as React from "react"
import { useLocation, useNavigate } from "react-router-dom"
import { useQuery } from "@apollo/client/react"
import { graphql } from "@/graphql"
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar"
import {
  ChevronRight,
  ChevronDown,
  Home,
  LayoutDashboard,
  Users,
  Clock,
  Star,
  FolderRoot,
  BookUser,
  CircleAlertIcon,
} from "lucide-react"
import { FilesystemTree } from "@/components/custom/FilesystemTreeView.tsx"
import PlatriumLogo from "../assets/PlatriumLogo.tsx"
import { Spinner } from "./ui/spinner.tsx"

// --- TYPES ---
export type StaticNavItem = {
  id: string
  label: string
  icon: React.ElementType
  path: string
}

export type FolderNode = {
  id: string
  parentId: string | null
  name: string
  hasChildren: boolean
  icon?: React.ElementType
  isLoading?: boolean
}

// --- STUB DATA ---
const TOP_NAV_ITEMS: StaticNavItem[] = [
  { id: "home", label: "Home", icon: Home, path: "/home" },
  {
    id: "projects",
    label: "Projects",
    icon: LayoutDashboard,
    path: "/projects",
  },
]

const BOTTOM_NAV_ITEMS: StaticNavItem[] = [
  { id: "shared", label: "Shared with me", icon: Users, path: "/shared" },
  { id: "recent", label: "Recent", icon: Clock, path: "/recent" },
  { id: "starred", label: "Starred", icon: Star, path: "/starred" },
]

/* Graph QL Queries */
const GET_DRIVES = graphql(`
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

// --- COMPONENTS ---

function StaticNavSection({ items }: { items: StaticNavItem[] }) {
  const location = useLocation()
  const navigate = useNavigate()

  return (
    <SidebarMenu>
      {items.map((item) => {
        const isActive = location.pathname.startsWith(item.path)
        const Icon = item.icon

        return (
          <SidebarMenuItem key={item.id}>
            <SidebarMenuButton
              isActive={isActive}
              onClick={() => navigate(item.path)}
            >
              <Icon />
              <span>{item.label}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        )
      })}
    </SidebarMenu>
  )
}

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const navigate = useNavigate()
  const location = useLocation()

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

  // Helper function for the generic tree (deferring sub-folder fetches for now)
  const getChildren = React.useCallback((parentId: string) => {
    return []
  }, [])

  return (
    <Sidebar {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem className="flex items-center">
            <div
              className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 px-2 py-2"
              onClick={() => navigate("/home")}
            >
              <div className="flex shrink-0 items-center justify-center">
                <PlatriumLogo className="!size-8" />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">Platrium</span>
                {/* TODO: Add Tenant Name here (Future GraphQL APIs) */}
                <span className="truncate text-xs"></span>
              </div>
            </div>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        {/* Top Static Links */}
        <SidebarGroup>
          <SidebarGroupContent>
            <StaticNavSection items={TOP_NAV_ITEMS} />
          </SidebarGroupContent>
        </SidebarGroup>

        {/* Unified Drives Group */}
        <SidebarGroup>
          <SidebarGroupLabel>Drives</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="group/tree">
              {/* 1. My Drive */}
              {loading ? (
                <div className="flex items-center gap-2 px-3 text-sm text-muted-foreground"><Spinner />Loading drives</div>
              ) : error ? (
                <div className="flex items-center gap-2 px-3 text-sm text-destructive"><CircleAlertIcon className="size-4" />Failed to Load Drives</div>
              ) : (
                <FilesystemTree
                  nodes={privateDrives}
                  getChildren={getChildren}
                  activeId={location.pathname.split("/").pop()}
                  onSelect={(id) => navigate(`/folder/${id}`)}
                />
              )}

              {/* 2. Shared Drives (Conditional) */}
              {sharedDrives.length > 0 && (
                <>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      onClick={() => setExpandedSharedDrives((prev) => !prev)}
                      style={{ paddingLeft: "8px" }}
                    >
                      {expandedSharedDrives ? (
                        <ChevronDown className="size-4 flex-shrink-0 cursor-pointer" />
                      ) : (
                        <ChevronRight className="size-4 flex-shrink-0 cursor-pointer" />
                      )}
                      <BookUser className="size-4 flex-shrink-0" />
                      <span className="truncate">Shared Drives</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  {expandedSharedDrives && (
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
                </>
              )}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {/* Bottom Static Links */}
        <SidebarGroup className="mt-auto">
          <SidebarGroupContent>
            <StaticNavSection items={BOTTOM_NAV_ITEMS} />
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}
