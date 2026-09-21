import React from "react"
import { cn } from "@/lib/utils"

export interface CircularProgressProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  value: number // 0 to 100
  size?: number
  strokeWidth?: number
  innerIcon?: React.ReactNode | React.ElementType
  iconClassName?: string
  trackClassName?: string
  indicatorClassName?: string
}

export function CircularProgress({
  value,
  size = 18,
  strokeWidth = 1.75,
  innerIcon,
  iconClassName = "h-3 w-3 text-muted-foreground group-hover/btn:text-destructive transition-colors",
  trackClassName = "opacity-20",
  indicatorClassName = "transition-all duration-300 ease-out",
  onClick,
  title,
  className = "",
  type = "button",
  ...props
}: CircularProgressProps) {
  const center = size / 2
  const radius = center - strokeWidth
  const circumference = 2 * Math.PI * radius
  const strokeDashoffset =
    circumference - (Math.min(100, Math.max(0, value)) / 100) * circumference

  const renderInnerIcon = () => {
    if (!innerIcon) return null

    const iconClasses = cn(
      "h-3 w-3 text-muted-foreground group-hover/btn:text-destructive transition-colors opacity-100 sm:opacity-0 sm:group-hover:opacity-100 group-hover/btn:opacity-100",
      iconClassName
    )

    // If passed an icon component reference (e.g., innerIcon={X})
    if (typeof innerIcon === "function" || (typeof innerIcon === "object" && "render" in (innerIcon as object))) {
      const IconComponent = innerIcon as React.ElementType
      return (
        <span className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <IconComponent className={iconClasses} />
        </span>
      )
    }

    // If passed a JSX element (e.g., innerIcon={<X />})
    return (
      <span className={cn("absolute inset-0 flex items-center justify-center pointer-events-none [&>svg]:h-3 [&>svg]:w-3 text-muted-foreground group-hover/btn:text-destructive transition-colors opacity-100 sm:opacity-0 sm:group-hover:opacity-100 group-hover/btn:opacity-100", iconClassName)}>
        {innerIcon}
      </span>
    )
  }

  const svgContent = (
    <>
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="-rotate-90 transform shrink-0 text-primary group-hover/btn:text-destructive transition-colors"
      >
        {/* Track */}
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="transparent"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className={trackClassName}
        />
        {/* Progress Indicator */}
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="transparent"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={strokeDashoffset}
          strokeLinecap="round"
          className={indicatorClassName}
        />
      </svg>
      {renderInnerIcon()}
    </>
  )

  if (onClick) {
    return (
      <button
        type={type}
        onClick={onClick}
        title={title}
        className={cn(
          "group/btn relative flex items-center justify-center rounded-full p-0 border-0 outline-none focus:outline-none focus:ring-0 focus-visible:ring-0 select-none bg-transparent hover:bg-destructive/10 transition-all cursor-pointer",
          className
        )}
        style={{ width: size, height: size }}
        {...props}
      >
        {svgContent}
      </button>
    )
  }

  return (
    <div
      title={title}
      className={cn("relative flex items-center justify-center rounded-full p-0 select-none", className)}
      style={{ width: size, height: size }}
    >
      {svgContent}
    </div>
  )
}
