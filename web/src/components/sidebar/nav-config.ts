import type { ElementType } from "react"
import { Clock, Home, ShieldCheck, Users, Trash2 } from "lucide-react"
import type { Permission } from "@/graphql/graphql"

export type NavItem = {
  id: string
  label: string
  icon: ElementType
  path: string
  /** Shown only to users holding this permission. */
  requires?: Permission
}

/** Which sidebar panel is showing. The URL decides, so deep links and back/forward just work. */
export type SidebarPanelId = "main" | "admin"

/** Panels left to right; the switcher slides between neighbours in this order. */
export const SIDEBAR_PANEL_ORDER: SidebarPanelId[] = ["main", "admin"]

export const ADMIN_PATH_PREFIX = "/admin"

export function panelFor(pathname: string): SidebarPanelId {
  return pathname === ADMIN_PATH_PREFIX ||
    pathname.startsWith(`${ADMIN_PATH_PREFIX}/`)
    ? "admin"
    : "main"
}

export const MAIN_NAV_ITEMS: NavItem[] = [
  { id: "home", label: "Home", icon: Home, path: "/home" },
  { id: "recent", label: "Recent", icon: Clock, path: "/recent" },
]

export const LOCATIONS_NAV_ITEMS: NavItem[] = [
  { id: "shared", label: "Shared with me", icon: Users, path: "/shared" },
  { id: "trash", label: "Trash", icon: Trash2, path: "/trash" },
]

/** The way into the admin console, shown at the bottom of the main panel. */
export const ADMIN_CONSOLE_ENTRY: NavItem = {
  id: "admin-console",
  label: "Admin console",
  icon: ShieldCheck,
  path: `${ADMIN_PATH_PREFIX}/users`,
  requires: "USERS_READ",
}

/** Admin console pages. Groups joins this list when group management exists. */
export const ADMIN_NAV_ITEMS: NavItem[] = [
  {
    id: "admin-users",
    label: "Users",
    icon: Users,
    path: `${ADMIN_PATH_PREFIX}/users`,
    requires: "USERS_READ",
  },
]
