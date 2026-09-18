import type { ComponentType, SVGProps, ReactNode } from "react"
import { cn } from "@/lib/utils"

export interface PlaceholderViewProps {
  icon?: ComponentType<SVGProps<SVGSVGElement>>
  title: string
  description?: ReactNode
  variant?: "default" | "error"
  action?: ReactNode
  className?: string
}

export function PlaceholderView({
  icon: Icon,
  title,
  description,
  variant = "default",
  action,
  className,
}: PlaceholderViewProps) {
  const renderDescription = () => {
    if (!description) return null
    if (typeof description === "string" && /<[a-z][\s\S]*>/i.test(description)) {
      return <span dangerouslySetInnerHTML={{ __html: description }} />
    }
    return description
  }

  return (
    <div
      className={cn(
        "flex h-full min-h-[50vh] w-full animate-in flex-col items-center justify-center p-8 text-center duration-300 fade-in",
        className
      )}
    >
      {Icon && (
        <div
          className={cn(
            "mb-4 flex size-20 items-center justify-center rounded-full",
            variant === "error"
              ? "bg-destructive/10 text-destructive"
              : "bg-muted/50 text-muted-foreground"
          )}
        >
          <Icon className="size-10" strokeWidth={1.5} />
        </div>
      )}
      <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
      {description && (
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          {renderDescription()}
        </p>
      )}
      {action && <div className="mt-6">{action}</div>}
    </div>
  )
}

