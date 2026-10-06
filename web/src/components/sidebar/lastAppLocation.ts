import { useEffect } from "react"
import { useLocation } from "react-router-dom"
import { panelFor } from "./nav-config"

const KEY = "platrium.lastAppLocation"
const FALLBACK = "/home"

/** Remembers the last page outside the admin console, so "Back to Platrium" returns there. */
export function useRememberAppLocation() {
  const { pathname, search } = useLocation()
  useEffect(() => {
    if (panelFor(pathname) !== "main") return
    try {
      sessionStorage.setItem(KEY, pathname + search)
    } catch {
      // Storage can be unavailable; falling back to home is fine.
    }
  }, [pathname, search])
}

export function getLastAppLocation(): string {
  try {
    return sessionStorage.getItem(KEY) || FALLBACK
  } catch {
    return FALLBACK
  }
}
