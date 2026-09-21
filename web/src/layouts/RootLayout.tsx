import { ApolloProvider } from "@apollo/client/react"
import { client } from "@/lib/apollo"
import { AppSidebar } from "@/components/app-sidebar"
import { AppBreadcrumbs } from "@/components/custom/AppBreadcrumbs"
import { Separator } from "@/components/ui/separator"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Outlet } from "react-router-dom"

export default function RootLayout() {
  return (
    <ApolloProvider client={client}>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator
              orientation="vertical"
              className="mr-2 data-[orientation=vertical]:h-full"
            />
            <AppBreadcrumbs />
          </header>
          <div className="flex flex-1 flex-col gap-4 p-4">
            <Outlet />
          </div>
        </SidebarInset>
      </SidebarProvider>
    </ApolloProvider>
  )
}
