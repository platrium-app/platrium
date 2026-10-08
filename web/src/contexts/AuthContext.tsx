import { createContext, useContext, useEffect, useState, type ReactNode } from "react"

export interface AuthUser {
  user_id: string
  tenant_id: string
  email: string
  /** The user's name as the server has it now. */
  display_name: string
  auth_kind?: "SESSION" | "DEVICE" | "APP"
}

interface AuthContextType {
  user: AuthUser | null
  isLoading: boolean
  error: Error | null
  refreshAuth: () => Promise<void>
}

const AuthContext = createContext<AuthContextType>({
  user: null,
  isLoading: true,
  error: null,
  refreshAuth: async () => { },
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetchMe = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const res = await fetch("/api/auth/me", {
        headers: {
          Accept: "application/json",
        },
      })

      if (!res.ok) {
        setUser(null)
        if (res.status !== 401) {
          throw new Error("Failed to fetch user profile")
        }
        return
      }

      const data = await res.json()
      setUser(data)
    } catch (err: any) {
      setError(err)
      setUser(null)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchMe()
  }, [])

  return (
    <AuthContext.Provider value={{ user, isLoading, error, refreshAuth: fetchMe }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
