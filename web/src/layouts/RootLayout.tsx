import { AppSidebar } from "@/components/sidebar/AppSidebar"
import { AppBreadcrumbs } from "@/components/custom/AppBreadcrumbs"
import { Separator } from "@/components/ui/separator"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Outlet, useLocation } from "react-router-dom"

/**
 * What identifies a page for the entrance animation. Moving between folders is
 * the same page showing different data, so it must not replay (or remount).
 */
function pageKey(pathname: string) {
  return pathname.replace(/^\/folder\/[^/]+/, "/folder")
}

export default function RootLayout() {
  const { pathname } = useLocation()
  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset className="flex h-screen flex-col overflow-hidden">
        <header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator
            orientation="vertical"
            className="mr-2 data-[orientation=vertical]:h-full"
          />
          <AppBreadcrumbs />
        </header>
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
          <div
            key={pageKey(pathname)}
            className="flex min-h-0 flex-1 animate-in flex-col duration-150 fade-in-0 slide-in-from-bottom-1 motion-reduce:animate-none"
          >
            <Outlet />
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
