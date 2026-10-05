import { Outlet, useLocation, useNavigate } from "react-router-dom"
import { AppBreadcrumbs } from "@/components/custom/AppBreadcrumbs"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import PlatriumLogo from "@/assets/PlatriumLogo"

// The frame for visitors who are not signed in and opened something shared with
// them by link: no sidebar, just a way to sign in.
export default function PublicLayout() {
  const navigate = useNavigate()
  const location = useLocation()

  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
        <div className="flex items-center gap-2 py-2">
          <PlatriumLogo className="size-6" />
          <span className="text-sm font-medium">Platrium</span>
        </div>
        <Separator orientation="vertical" className="mx-2 !h-full" />
        <div className="min-w-0 flex-1">
          <AppBreadcrumbs />
        </div>
        <Button variant="outline" size="sm" onClick={() => navigate("/login", { state: { from: location } })}>
          Sign in
        </Button>
      </header>
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
        <Outlet />
      </div>
    </div>
  )
}
