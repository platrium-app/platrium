import type { ComponentProps } from "react"
import { useLocation } from "react-router-dom"
import type { Sidebar } from "@/components/ui/sidebar"
import { AdminSidebarPanel } from "./AdminSidebarPanel"
import { MainSidebarPanel } from "./MainSidebarPanel"
import { SidebarPanelSwitcher } from "./SidebarPanelSwitcher"
import { SidebarShell } from "./SidebarShell"
import {
  ADMIN_CONSOLE_ENTRY,
  SIDEBAR_PANEL_ORDER,
  panelFor,
} from "./nav-config"
import { useRememberAppLocation } from "./lastAppLocation"
import { usePermissions } from "@/hooks/usePermissions"

/**
 * The app's sidebar. Which panel shows follows the URL: everything under
 * /admin shows the admin console's, and the rest shows the main one.
 */
export function AppSidebar(props: ComponentProps<typeof Sidebar>) {
  const { pathname } = useLocation()
  const { can, loading } = usePermissions()
  useRememberAppLocation()

  // Someone who may not use the admin console never sees its sidebar, even on a
  // deep link; the page itself explains. While permissions load, follow the URL.
  const adminAllowed = loading || can(ADMIN_CONSOLE_ENTRY.requires!)
  const active =
    panelFor(pathname) === "admin" && !adminAllowed
      ? "main"
      : panelFor(pathname)

  return (
    <SidebarShell {...props}>
      <SidebarPanelSwitcher
        active={active}
        order={SIDEBAR_PANEL_ORDER}
        panels={{ main: <MainSidebarPanel />, admin: <AdminSidebarPanel /> }}
      />
    </SidebarShell>
  )
}
