import { useState } from "react"
import { useLocation } from "react-router-dom"
import { AuthLayout } from "@/layouts/AuthLayout"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldSeparator,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useQuery } from "@apollo/client/react"
import { graphql } from "@/graphql"
import type { GetAuthConfigQuery } from "@/graphql/graphql"

export type IdpProviderInfo = GetAuthConfigQuery["tenantAuthConfig"]["providers"][0]

const GET_AUTH_CONFIG = graphql(`
  query GetAuthConfig {
    tenantAuthConfig {
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

export function SingleTenantLogin({
  tenantAlias,
  providers = [],
}: {
  tenantAlias?: string
  providers?: IdpProviderInfo[]
}) {
  const [step, setStep] = useState<"credentials" | "2fa">("credentials")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [mfaCode, setMfaCode] = useState("")
  const [challengeToken, setChallengeToken] = useState("")
  const [mfaMethod, setMfaMethod] = useState("")
  const [errorMsg, setErrorMsg] = useState("")
  const location = useLocation()
  
  const getReturnUrl = () => {
    const from = location.state?.from
    return from ? from.pathname + (from.search || "") : "/home"
  }

  // If providers are passed down (e.g. from an EE wrapper), use them.
  // Otherwise, fetch the CE auth config.
  const { data, loading, error } = useQuery(GET_AUTH_CONFIG, {
    skip: providers.length > 0, // skip CE fetch if providers were injected
  })

  if (!providers.length && loading) {
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

  if (!providers.length && (error || !data)) {
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

  const activeProviders: IdpProviderInfo[] = providers.length > 0 
    ? providers 
    : (data?.tenantAuthConfig?.providers || [])
  const activeName: string = providers.length > 0 
    ? (tenantAlias || "")
    : (data?.tenantAuthConfig?.name || "")

  const localProvider = activeProviders.find((p: IdpProviderInfo) => p.type === "LOCAL")
  const externalProviders = activeProviders.filter((p: IdpProviderInfo) => p.type !== "LOCAL")

  const handleLocalSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setErrorMsg("")

    if (step === "credentials") {
      try {
        const res = await fetch("/api/auth/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            idp_id: localProvider?.id,
            email,
            password,
          }),
        })
        const data = await res.json()

        if (data.status === "CHALLENGE_REQUIRED") {
          setChallengeToken(data.challenge_token || "")
          setMfaMethod(data.next_step || "MFA_TOTP")
          setStep("2fa")
        } else if (res.ok && data.status === "SUCCESS") {
          window.location.href = getReturnUrl()
        } else {
          setErrorMsg(data.message || "Invalid credentials")
        }
      } catch (err) {
        setErrorMsg("Network error during login")
      }
    } else {
      try {
        const res = await fetch("/api/auth/mfa/verify", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            challenge_token: challengeToken,
            method: mfaMethod,
            code: mfaCode,
          }),
        })
        const data = await res.json()
        if (res.ok && data.status === "SUCCESS") {
          window.location.href = getReturnUrl()
        } else {
          setErrorMsg(data.message || "Invalid MFA code")
        }
      } catch (err) {
        setErrorMsg("Network error verifying MFA")
      }
    }
  }

  const handleIdpClick = (idpId: string) => {
    window.location.href = `/api/auth/login?idp=${encodeURIComponent(idpId)}`
  }

  return (
    <AuthLayout>
      <Card>
        <CardHeader className="text-center">
          <CardTitle className="text-xl">
            {activeName ? `Sign in to ${activeName}` : "Welcome back"}
          </CardTitle>
          <CardDescription>
            {step === "2fa"
              ? "Enter your 2FA authentication code"
              : "Login to your account"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {errorMsg && (
            <div className="mb-4 rounded-md bg-destructive/15 p-3 text-sm text-destructive">
              {errorMsg}
            </div>
          )}
          <form onSubmit={handleLocalSubmit}>
            <FieldGroup>
              {step === "credentials" ? (
                <>
                  {externalProviders.length > 0 && (
                    <>
                      <Field>
                        {externalProviders.map((p) => (
                          <Button
                            key={p.id}
                            variant="outline"
                            type="button"
                            onClick={() => handleIdpClick(p.id)}
                            className="w-full mb-2"
                          >
                            Login with {p.name}
                          </Button>
                        ))}
                      </Field>
                      {localProvider && (
                        <FieldSeparator className="*:data-[slot=field-separator-content]:bg-card">
                          Or continue with
                        </FieldSeparator>
                      )}
                    </>
                  )}
                  {localProvider && (
                    <>
                      <Field>
                        <FieldLabel htmlFor="email">Email</FieldLabel>
                        <Input
                          id="email"
                          type="email"
                          placeholder="name@example.com"
                          required
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                        />
                      </Field>
                      <Field>
                        <div className="flex items-center">
                          <FieldLabel htmlFor="password">Password</FieldLabel>
                          <a
                            href="#"
                            className="ml-auto text-sm underline-offset-4 hover:underline"
                          >
                            Forgot your password?
                          </a>
                        </div>
                        <Input
                          id="password"
                          type="password"
                          required
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                        />
                      </Field>
                      <Field>
                        <Button type="submit">Login</Button>
                        <FieldDescription className="text-center">
                          Don&apos;t have an account? <a href="#">Sign up</a>
                        </FieldDescription>
                      </Field>
                    </>
                  )}
                </>
              ) : (
                <>
                  <Field>
                    <FieldLabel htmlFor="mfaCode">
                      Authentication Code
                    </FieldLabel>
                    <Input
                      id="mfaCode"
                      type="text"
                      placeholder="123456"
                      maxLength={6}
                      required
                      autoFocus
                      value={mfaCode}
                      onChange={(e) => setMfaCode(e.target.value)}
                    />
                  </Field>
                  <Field>
                    <Button type="submit">Verify Code</Button>
                    <Button
                      variant="ghost"
                      type="button"
                      onClick={() => {
                        setStep("credentials")
                        setMfaCode("")
                        setErrorMsg("")
                      }}
                      className="mt-2"
                    >
                      Back to login
                    </Button>
                  </Field>
                </>
              )}
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}


