import React, { createContext, useContext, useState, useEffect } from "react"

export type BreadcrumbItemType = {
  id?: string
  label: string
  href?: string
  icon?: React.ElementType
}

type BreadcrumbContextType = {
  breadcrumbs: BreadcrumbItemType[]
  setBreadcrumbs: (items: BreadcrumbItemType[]) => void
}

const BreadcrumbContext = createContext<BreadcrumbContextType | undefined>(
  undefined
)

export function BreadcrumbProvider({ children }: { children: React.ReactNode }) {
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItemType[]>([])

  return (
    <BreadcrumbContext.Provider value={{ breadcrumbs, setBreadcrumbs }}>
      {children}
    </BreadcrumbContext.Provider>
  )
}

export function useBreadcrumbs() {
  const context = useContext(BreadcrumbContext)
  if (!context) {
    throw new Error("useBreadcrumbs must be used within a BreadcrumbProvider")
  }
  return context
}

export function useSetBreadcrumbs(items: BreadcrumbItemType[]) {
  const { setBreadcrumbs } = useBreadcrumbs()

  useEffect(() => {
    setBreadcrumbs(items)
  }, [JSON.stringify(items), setBreadcrumbs])
}
