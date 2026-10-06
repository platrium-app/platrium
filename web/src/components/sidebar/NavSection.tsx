import { useLocation, useNavigate } from "react-router-dom"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { usePermissions } from "@/hooks/usePermissions"
import type { NavItem } from "./nav-config"

/** A list of navigation buttons, leaving out the ones the user lacks the permission for. */
export function NavSection({ items }: { items: NavItem[] }) {
  const location = useLocation()
  const navigate = useNavigate()
  const { can } = usePermissions()

  return (
    <SidebarMenu>
      {items
        .filter((item) => !item.requires || can(item.requires))
        .map((item) => {
          const isActive =
            location.pathname === item.path ||
            location.pathname.startsWith(`${item.path}/`)
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
