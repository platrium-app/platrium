import { type ReactNode } from "react"
import { ShieldAlertIcon } from "lucide-react"
import type { Permission } from "@/graphql/graphql"
import { usePermissions } from "@/hooks/usePermissions"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { Spinner } from "@/components/ui/spinner"

/**
 * Renders its children only for users holding the permission. This is a
 * courtesy, not a defence: every admin field is also gated on the server.
 */
export function RequirePermission({
  permission,
  children,
}: {
  permission: Permission
  children: ReactNode
}) {
  const { can, loading } = usePermissions()

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Spinner className="size-6 text-muted-foreground" />
      </div>
    )
  }
  if (!can(permission)) {
    return (
      <PlaceholderView
        icon={ShieldAlertIcon}
        title="You don't have access to this page"
        description="Ask an administrator of your organization if you need it."
      />
    )
  }
  return <>{children}</>
}
