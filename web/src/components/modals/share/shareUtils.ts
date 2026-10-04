// Small helpers for the sharing dialog.

export type SubjectType = "USER" | "GROUP" | "TENANT" | "PUBLIC" | (string & {})

export interface Subject {
  type: SubjectType
  id: string
  name: string
  email?: string | null
}

/** Up to two initials for an avatar. */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return "?"
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

/** "VIEWER" -> "Viewer", for roles the server did not label. */
export function humanizeRole(role: string): string {
  return role
    .toLowerCase()
    .split("_")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ")
}

/** The GraphQL DateTime scalar arrives as an ISO string; anything else is treated as absent. */
export function asIso(value: unknown): string | null {
  return typeof value === "string" ? value : null
}

export function formatExpiry(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  })
}

/** A date from <input type="date"> (yyyy-mm-dd) as the end of that day, in ISO form. */
export function endOfDayIso(date: string): string | null {
  if (!date) return null
  const d = new Date(`${date}T23:59:59`)
  return Number.isNaN(d.getTime()) ? null : d.toISOString()
}

/** An ISO time as the yyyy-mm-dd value <input type="date"> expects. */
export function isoToDateInput(iso?: string | null): string {
  if (!iso) return ""
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** The server's stable error code, whichever shape Apollo wraps the error in. */
export function errorCode(err: unknown): string | undefined {
  const e = err as {
    errors?: { extensions?: { code?: string } }[]
    graphQLErrors?: { extensions?: { code?: string } }[]
  }
  return (
    e?.errors?.[0]?.extensions?.code ?? e?.graphQLErrors?.[0]?.extensions?.code
  )
}

/** A message a person can act on. */
export function friendlyError(err: unknown): string {
  switch (errorCode(err)) {
    case "FORBIDDEN":
      return (err as Error)?.message?.includes("public sharing")
        ? "Your organization does not allow public sharing."
        : "You don't have permission to do that."
    case "NOT_FOUND":
      return "That item or person no longer exists."
    case "UNAUTHENTICATED":
      return "Your session has ended. Sign in again."
    case "CONFLICT":
      return "That already exists."
  }
  const message = (err as Error)?.message
  return message ? message : "Something went wrong. Try again."
}

/** The link that opens an item: its own page. */
export function itemLink(id: string, type: "FILE" | "FOLDER"): string {
  return `${window.location.origin}/${type === "FOLDER" ? "folder" : "file"}/${id}`
}
