import type { ComponentProps, ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import {
  Sidebar,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar"
import { NavUser } from "@/components/custom/SignedInUserControl"
import PlatriumLogo from "@/assets/PlatriumLogo"
import { useAuth } from "@/contexts/AuthContext"

/** The parts of the sidebar that stay put: the logo on top, the signed-in user at the bottom. */
export function SidebarShell({
  children,
  ...props
}: ComponentProps<typeof Sidebar> & { children: ReactNode }) {
  const navigate = useNavigate()
  const { user } = useAuth()

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

      {children}

      <SidebarFooter>
        {user && (
          <NavUser
            user={{
              name: user.email.split("@")[0] || "User",
              email: user.email,
              avatar: "",
            }}
          />
        )}
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
