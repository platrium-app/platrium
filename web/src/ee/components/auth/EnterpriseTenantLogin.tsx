import { useQuery } from "@apollo/client/react"
import { AuthLayout } from "@/layouts/AuthLayout"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"
import { SingleTenantLogin, type IdpProviderInfo } from "@/components/auth/SingleTenantLogin"
import { graphql } from "@/graphql"

const GET_EE_AUTH_CONFIG = graphql(`
  query GetEEAuthConfig($alias: String!) {
    tenantAuthConfigByAlias(alias: $alias) {
      tenantId
      name
      alias
      defaultIdpId
      providers {
        id
        name
        type
      }
    }
  }
`)

export default function EnterpriseTenantLogin({ tenantAlias }: { tenantAlias: string }) {
  const { data, loading, error } = useQuery(GET_EE_AUTH_CONFIG, {
    variables: { alias: tenantAlias },
  })

  if (loading) {
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

  if (error || !data?.tenantAuthConfigByAlias) {
    return (
      <AuthLayout>
        <Card>
          <CardHeader className="text-center">
            <CardTitle className="text-xl text-destructive">Error Loading Configuration</CardTitle>
            <CardDescription>{error?.message || "Failed to load authentication configuration"}</CardDescription>
          </CardHeader>
        </Card>
      </AuthLayout>
    )
  }

  const providers: IdpProviderInfo[] = data.tenantAuthConfigByAlias.providers || []
  const name = data.tenantAuthConfigByAlias.name

  return (
    <SingleTenantLogin
      tenantAlias={name}
      providers={providers}
    />
  )
}
