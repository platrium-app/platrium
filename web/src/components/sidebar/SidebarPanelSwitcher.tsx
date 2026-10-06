import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

/**
 * Shows one of several panels and slides between them. Panels sit side by side
 * in `order`; the active one is in view and the rest are parked left or right
 * of it, so moving between neighbours slides in the natural direction.
 *
 * Parked panels are `inert`: they cannot be focused, clicked or read by a
 * screen reader. The slide is skipped for people who ask for reduced motion.
 */
export function SidebarPanelSwitcher<Id extends string>({
  active,
  order,
  panels,
  className,
}: {
  active: Id
  order: readonly Id[]
  panels: Record<Id, ReactNode>
  className?: string
}) {
  const activeIndex = order.indexOf(active)

  return (
    <div className={cn("relative min-h-0 flex-1 overflow-hidden", className)}>
      {order.map((id, index) => {
        const isActive = id === active
        return (
          <div
            key={id}
            inert={!isActive}
            aria-hidden={!isActive}
            style={{ transform: `translateX(${(index - activeIndex) * 100}%)` }}
            className="absolute inset-0 flex flex-col transition-transform duration-200 ease-out motion-reduce:transition-none"
          >
            {panels[id]}
          </div>
        )
      })}
    </div>
  )
}
