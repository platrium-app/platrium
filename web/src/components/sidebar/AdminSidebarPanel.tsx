import { ArrowLeft } from "lucide-react"
import { useNavigate } from "react-router-dom"
import {
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { NavSection } from "./NavSection"
import { ADMIN_NAV_ITEMS, ADMIN_SIGN_IN_NAV_ITEMS } from "./nav-config"
import { getLastAppLocation } from "./lastAppLocation"

/** The sidebar inside the admin console. */
export function AdminSidebarPanel() {
  const navigate = useNavigate()

  return (
    <SidebarContent>
      <SidebarGroup>
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton onClick={() => navigate(getLastAppLocation())}>
                <ArrowLeft />
                <span>Back to Platrium</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>

      <SidebarGroup>
        <SidebarGroupLabel>Users &amp; Groups</SidebarGroupLabel>
        <SidebarGroupContent>
          <NavSection items={ADMIN_NAV_ITEMS} />
        </SidebarGroupContent>
      </SidebarGroup>

      <SidebarGroup>
        <SidebarGroupLabel>Sign-in</SidebarGroupLabel>
        <SidebarGroupContent>
          <NavSection items={ADMIN_SIGN_IN_NAV_ITEMS} />
        </SidebarGroupContent>
      </SidebarGroup>
    </SidebarContent>
  )
}
