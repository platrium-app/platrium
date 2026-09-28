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
  BreadcrumbEllipsis,
} from "@/components/ui/breadcrumb"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

export function AppBreadcrumbs() {
  const { breadcrumbs } = useBreadcrumbs()

  if (!breadcrumbs || breadcrumbs.length === 0) {
    return null
  }

  const ITEMS_TO_DISPLAY = 3

  const renderBreadcrumbItem = (item: any, isLast: boolean, key: string) => {
    const Icon = item.icon
    return (
      <React.Fragment key={key}>
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
  }

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {breadcrumbs.length > ITEMS_TO_DISPLAY ? (
          <>
            {renderBreadcrumbItem(breadcrumbs[0], false, "first")}
            <React.Fragment key="ellipsis">
              <BreadcrumbItem>
                <DropdownMenu>
                  <DropdownMenuTrigger className="flex items-center gap-1 outline-none">
                    <BreadcrumbEllipsis className="h-4 w-4" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    {breadcrumbs.slice(1, -2).map((item, index) => {
                      const Icon = item.icon
                      return (
                        <DropdownMenuItem 
                          key={item.id || index} 
                          render={<Link to={item.href || "#"} />}
                          className="flex items-center gap-2"
                        >
                          {Icon && <Icon className="size-4 shrink-0 text-muted-foreground" />}
                          {item.label}
                        </DropdownMenuItem>
                      )
                    })}
                  </DropdownMenuContent>
                </DropdownMenu>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
            </React.Fragment>
            {renderBreadcrumbItem(breadcrumbs[breadcrumbs.length - 2], false, "second-to-last")}
            {renderBreadcrumbItem(breadcrumbs[breadcrumbs.length - 1], true, "last")}
          </>
        ) : (
          breadcrumbs.map((item, index) =>
            renderBreadcrumbItem(item, index === breadcrumbs.length - 1, item.id || item.href || item.label || String(index))
          )
        )}
      </BreadcrumbList>
    </Breadcrumb>
  )
}


