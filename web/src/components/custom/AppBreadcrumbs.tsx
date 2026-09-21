import * as React from "react"
import { Link } from "react-router-dom"
import { useBreadcrumbs } from "@/contexts/BreadcrumbContext"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { cn } from "@/lib/utils"

export function AppBreadcrumbs() {
  const { breadcrumbs } = useBreadcrumbs()

  if (!breadcrumbs || breadcrumbs.length === 0) {
    return null
  }

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {breadcrumbs.map((item, index) => {
          const isLast = index === breadcrumbs.length - 1
          const Icon = item.icon

          return (
            <React.Fragment key={item.id || item.href || item.label || index}>
              <BreadcrumbItem>
                {item.href ? (
                  <BreadcrumbLink
                    render={
                      <Link
                        to={item.href}
                        className={cn(
                          "inline-flex items-center gap-1.5 transition-colors",
                          isLast
                            ? "font-normal text-foreground"
                            : "text-muted-foreground hover:text-foreground"
                        )}
                      />
                    }
                  >
                    {Icon && <Icon className="size-4 shrink-0" />}
                    <span>{item.label}</span>
                  </BreadcrumbLink>
                ) : (
                  <BreadcrumbPage className="inline-flex items-center gap-1.5">
                    {Icon && <Icon className="size-4 shrink-0" />}
                    <span>{item.label}</span>
                  </BreadcrumbPage>
                )}
              </BreadcrumbItem>
              {!isLast && <BreadcrumbSeparator />}
            </React.Fragment>
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}


