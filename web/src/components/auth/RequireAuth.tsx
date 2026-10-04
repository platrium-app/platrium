import { type ReactNode } from "react"
import { Navigate, useLocation } from "react-router-dom"
import { useAuth } from "@/contexts/AuthContext"
import { AuthLayout } from "@/layouts/AuthLayout"
import { Spinner } from "@/components/ui/spinner"
import { Card, CardHeader, CardTitle } from "@/components/ui/card"

export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, isLoading } = useAuth()
  const location = useLocation()

  if (isLoading) {
    return (
      <AuthLayout>
        <Card>
          <CardHeader className="text-center">
            <CardTitle className="text-xl flex justify-center">
              <Spinner />
            </CardTitle>
          </CardHeader>
        </Card>
      </AuthLayout>
    )
  }

  if (!user) {
    // Redirect to the /login page, but save the current location they were
    // trying to go to when they were redirected. This allows us to send them
    // along to that page after they login, which is a nicer user experience
    // than dropping them off on the home page.
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}
