import { useState } from "react"
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

export interface IdpProviderInfo {
  id: string
  name: string
  type: string
}

export function SingleTenantLogin({
  tenantAlias,
  providers = [],
}: {
  tenantAlias?: string
  providers?: IdpProviderInfo[]
}) {
  const [step, setStep] = useState<"credentials" | "2fa">("credentials")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [mfaCode, setMfaCode] = useState("")
  const [mfaToken, setMfaToken] = useState("")

  const localProvider = providers.find((p) => p.type === "LOCAL")
  const externalProviders = providers.filter((p) => p.type !== "LOCAL")

  const handleLocalSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (step === "credentials") {
      try {
        const res = await fetch("/api/auth/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            idp_id: localProvider?.id || "idp_local_default",
            username,
            password,
          }),
        })
        const data = await res.json()
        if (data.status === "MFA_REQUIRED") {
          setMfaToken(data.mfa_token || "")
          setStep("2fa")
        } else if (res.ok) {
          window.location.href = "/home"
        }
      } catch (err) {
        console.error("Login error:", err)
      }
    } else {
      try {
        const res = await fetch("/api/auth/mfa/verify", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            mfa_token: mfaToken,
            code: mfaCode,
          }),
        })
        if (res.ok) {
          window.location.href = "/home"
        }
      } catch (err) {
        console.error("MFA error:", err)
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
            {tenantAlias ? `Sign in to ${tenantAlias}` : "Welcome back"}
          </CardTitle>
          <CardDescription>
            {step === "2fa"
              ? "Enter your 2FA authentication code"
              : "Login to your account"}
          </CardDescription>
        </CardHeader>
        <CardContent>
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
                      <FieldSeparator className="*:data-[slot=field-separator-content]:bg-card">
                        Or continue with
                      </FieldSeparator>
                    </>
                  )}
                  <Field>
                    <FieldLabel htmlFor="username">Username</FieldLabel>
                    <Input
                      id="username"
                      type="text"
                      placeholder="username"
                      required
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
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
                      onClick={() => setStep("credentials")}
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


