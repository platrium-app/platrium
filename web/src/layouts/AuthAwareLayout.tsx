import { AuthLayout } from "@/layouts/AuthLayout"
import { useAuth } from "@/contexts/AuthContext"
import { Spinner } from "@/components/ui/spinner"
import { Card, CardHeader, CardTitle } from "@/components/ui/card"
import PublicLayout from "./PublicLayout"
import RootLayout from "./RootLayout"

// For pages anyone may try to open. Signed-in users get the full app; visitors get
// a bare frame, and whatever they can see is decided by the server, not here.
export default function AuthAwareLayout() {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return (
      <AuthLayout>
        <Card>
          <CardHeader className="text-center">
            <CardTitle className="flex justify-center text-xl">
              <Spinner />
            </CardTitle>
          </CardHeader>
        </Card>
      </AuthLayout>
    )
  }

  return user ? <RootLayout /> : <PublicLayout />
}
