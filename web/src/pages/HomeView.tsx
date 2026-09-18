import { useSetBreadcrumbs } from "@/contexts/BreadcrumbContext"
import { Home } from "lucide-react"

export default function HomeView() {
  useSetBreadcrumbs([{ label: "Home", icon: Home }])

  return (
    <div className="flex flex-col gap-2 p-2">
      <h1 className="text-2xl font-bold tracking-tight">Welcome to Platrium</h1>
      <p className="text-muted-foreground">Select a drive or folder from the sidebar to start browsing your assets.</p>
    </div>
  )
}

