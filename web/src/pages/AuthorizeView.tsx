import { useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { AuthLayout } from "@/layouts/AuthLayout"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/contexts/AuthContext"

const PLATFORM_LABELS: Record<string, string> = {
  IOS: "iPhone or iPad",
  MACOS: "Mac",
  ANDROID: "Android device",
}

/** Where the approval will be sent, shown so users can spot a lookalike app. */
function describeTarget(redirectUri: string): string | null {
  try {
    const u = new URL(redirectUri)
    if (u.protocol === "http:" || u.protocol === "https:") return u.host
    return `${u.protocol}//${u.host}`
  } catch {
    return null
  }
}

/**
 * Consent screen for native apps and CLIs. They open this page in a browser
 * with their PKCE challenge; the normal login runs first (RequireAuth), then
 * the user approves and is sent back to the app with a one-time code.
 */
export default function AuthorizeView() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState("")

  const redirectUri = params.get("redirect_uri") ?? ""
  const challenge = params.get("code_challenge") ?? ""
  const name = params.get("name")?.trim() ?? ""
  const platform = params.get("platform")?.toUpperCase() ?? ""
  const target = describeTarget(redirectUri)

  const invalid = !target || !challenge || !name

  const approve = async () => {
    setBusy(true)
    setError("")
    try {
      const res = await fetch("/api/auth/authorize", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          redirect_uri: redirectUri,
          code_challenge: challenge,
          state: params.get("state") ?? undefined,
          name,
          platform: platform || undefined,
          app_version: params.get("app_version") ?? undefined,
        }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError(data.message || "This request could not be approved.")
        setBusy(false)
        return
      }
      setDone(true)
      window.location.href = data.redirect_to
    } catch {
      setError("Network error. Please try again.")
      setBusy(false)
    }
  }

  if (invalid) {
    return (
      <AuthLayout>
        <Card>
          <CardHeader className="text-center">
            <CardTitle className="text-xl text-destructive">Invalid request</CardTitle>
            <CardDescription>
              This sign-in link is missing information. Please start again from the app.
            </CardDescription>
          </CardHeader>
        </Card>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <Card>
        <CardHeader className="text-center">
          <CardTitle className="text-xl">Allow “{name}” to access Platrium?</CardTitle>
          <CardDescription>
            {platform
              ? `Sign in on your ${PLATFORM_LABELS[platform] ?? platform}`
              : "Connect this app"}{" "}
            as <span className="font-medium">{user?.email}</span>.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">
            It will be able to see and change everything you can. You can remove it any time
            under Devices &amp; Apps.
          </p>
          <p className="text-xs text-muted-foreground">
            You will be sent back to <span className="font-mono">{target}</span>.
          </p>
          {error && (
            <div className="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
          )}
          {done ? (
            <p className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
              <Spinner /> Returning to {name}. You can close this window.
            </p>
          ) : (
            <div className="flex flex-col gap-2">
              <Button onClick={approve} disabled={busy}>
                {busy ? <Spinner /> : "Allow"}
              </Button>
              <Button variant="ghost" onClick={() => navigate("/home")} disabled={busy}>
                Cancel
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
