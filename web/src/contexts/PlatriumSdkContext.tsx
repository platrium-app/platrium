import React, { createContext, useContext, useState, useMemo } from "react"
import { PlatriumClient } from "platrium-sdk"

interface SdkContextType {
  client: PlatriumClient
  serverUrl: string
  setServerUrl: (url: string) => void
}

const PlatriumSdkContext = createContext<SdkContextType | null>(null)

export function usePlatriumSdk() {
  const context = useContext(PlatriumSdkContext)
  if (!context) {
    throw new Error("usePlatriumSdk must be used within a PlatriumSdkProvider")
  }
  return context.client
}

export function usePlatriumSdkContext() {
  const context = useContext(PlatriumSdkContext)
  if (!context) {
    throw new Error("usePlatriumSdkContext must be used within a PlatriumSdkProvider")
  }
  return context
}

export function PlatriumSdkProvider({
  defaultUrl = "http://localhost:3000/api",
  children,
}: {
  defaultUrl?: string
  children: React.ReactNode
}) {
  // TODO: Future support needed for multiple servers, make this an array with UUID or smth?
  const [serverUrl, setServerUrl] = useState(defaultUrl)

  // Re-instantiate the client whenever the serverUrl changes
  const client = useMemo(() => new PlatriumClient(serverUrl), [serverUrl])

  return (
    <PlatriumSdkContext.Provider value={{ client, serverUrl, setServerUrl }}>
      {children}
    </PlatriumSdkContext.Provider>
  )
}
