// The engine sends people back to the login page with `?error=<code>` when an
// external sign-in does not complete. These are the sentences they see; the
// detail stays in the server log.
const MESSAGES: Record<string, string> = {
  sso_denied: "The sign-in was cancelled or declined by your provider.",
  sso_expired: "That sign-in link expired or was already used. Please try again.",
  sso_no_account:
    "You don't have an account here yet. Ask an administrator to add you, or sign in with a different account.",
  sso_disabled:
    "This account, or the sign-in method, has been disabled. Contact your administrator.",
  sso_failed: "Sign-in with your provider failed. Please try again.",
}

/** A message for an `error` code from the login redirect; empty when there is none. */
export function ssoErrorMessage(code: string | null): string {
  if (!code) return ""
  return MESSAGES[code] ?? MESSAGES.sso_failed
}
