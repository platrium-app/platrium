import { lazy, Suspense } from "react"
import { useParams } from "react-router-dom"
import { useServerInfo } from "@/contexts/ServerInfoContext"
import { SingleTenantLogin } from "@/components/auth/SingleTenantLogin"
import { AuthLayout } from "@/layouts/AuthLayout"
import { Card, CardHeader, CardTitle } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"

// Dynamically import the EE component so it gets completely tree-shaken
// out of the CE build if VITE_EDITION !== 'EE'
const EECompanyAliasLogin = import.meta.env.VITE_EDITION === 'EE'
  ? lazy(() => import('../ee/pages/CompanyAliasLogin'))
  : null

function AuthLoading() {
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

export default function LoginRouter() {
  const { alias } = useParams<{ alias?: string }>()
  const { info, loading } = useServerInfo()

  if (loading) {
    return <AuthLoading />
  }

  // 1. If explicit tenant alias is in URL (/login/:alias), show login form for that tenant
  if (alias) {
    return <SingleTenantLogin tenantAlias={alias} />
  }

  // 2. If server has multi-tenancy enabled, prompt for company alias at /login
  if (info?.isMultiTenant && EECompanyAliasLogin) {
    return (
      <Suspense fallback={<AuthLoading />}>
        <EECompanyAliasLogin />
      </Suspense>
    )
  }

  // 3. Otherwise (single-tenant CE), show native default tenant login form
  return <SingleTenantLogin />
}


