import * as React from "react"
import { cn } from "@/lib/utils"

export interface SelectionAreaProps {
  children: React.ReactNode
  selectedIds: Set<string>
  onSelectionChange: (selectedIds: Set<string>) => void
  itemSelector?: string
  className?: string
  disabled?: boolean
}

interface BoxRect {
  startX: number
  startY: number
  currentX: number
  currentY: number
  startClientX: number
  startClientY: number
  currentClientX: number
  currentClientY: number
}

export function SelectionArea({
  children,
  selectedIds,
  onSelectionChange,
  itemSelector = "[data-selection-id]",
  className,
  disabled = false,
}: SelectionAreaProps) {
  const containerRef = React.useRef<HTMLDivElement>(null)
  const [box, setBox] = React.useState<BoxRect | null>(null)
  const [isSelecting, setIsSelecting] = React.useState(false)
  const initialSelectedRef = React.useRef<Set<string>>(new Set())

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (disabled || e.button !== 0) return

    const target = e.target as HTMLElement
    // Ignore clicks on buttons, inputs, dropdown menus, context triggers or interactive controls
    if (
      target.closest(
        'button, a, input, select, textarea, [role="menu"], [role="menuitem"], [data-prevent-selection-drag]'
      )
    ) {
      return
    }

    const container = containerRef.current
    if (!container) return

    const containerRect = container.getBoundingClientRect()
    const startX = e.clientX - containerRect.left + container.scrollLeft
    const startY = e.clientY - containerRect.top + container.scrollTop

    // Save initial selection if Shift or Cmd/Ctrl key is held
    if (e.shiftKey || e.metaKey || e.ctrlKey) {
      initialSelectedRef.current = new Set(selectedIds)
    } else {
      initialSelectedRef.current = new Set()
      // If clicking directly on an empty space without modifier keys, clear selection
      if (!target.closest(itemSelector)) {
        onSelectionChange(new Set())
      } else {
        // If clicking down on an item, preserve current selection for item click handler unless dragged
        initialSelectedRef.current = new Set(selectedIds)
      }
    }

    setBox({
      startX,
      startY,
      currentX: startX,
      currentY: startY,
      startClientX: e.clientX,
      startClientY: e.clientY,
      currentClientX: e.clientX,
      currentClientY: e.clientY,
    })
  }

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!box || !containerRef.current) return

    const container = containerRef.current
    const containerRect = container.getBoundingClientRect()
    const currentX = e.clientX - containerRect.left + container.scrollLeft
    const currentY = e.clientY - containerRect.top + container.scrollTop

    const updatedBox: BoxRect = {
      ...box,
      currentX,
      currentY,
      currentClientX: e.clientX,
      currentClientY: e.clientY,
    }

    setBox(updatedBox)

    const deltaX = Math.abs(e.clientX - box.startClientX)
    const deltaY = Math.abs(e.clientY - box.startClientY)

    // Threshold to prevent accidental marquee trigger on plain clicks/double-clicks
    if (!isSelecting && (deltaX > 4 || deltaY > 4)) {
      setIsSelecting(true)
      try {
        container.setPointerCapture(e.pointerId)
      } catch {
        // Pointer capture fallback
      }
    }

    if (isSelecting || deltaX > 4 || deltaY > 4) {
      // Calculate viewport drag bounding box
      const boxLeft = Math.min(box.startClientX, e.clientX)
      const boxTop = Math.min(box.startClientY, e.clientY)
      const boxRight = Math.max(box.startClientX, e.clientX)
      const boxBottom = Math.max(box.startClientY, e.clientY)

      // Find all elements matching itemSelector inside container
      const itemElements = container.querySelectorAll(itemSelector)
      const newSelected = new Set(initialSelectedRef.current)

      itemElements.forEach((el) => {
        const id = el.getAttribute("data-selection-id")
        if (!id) return

        const itemRect = el.getBoundingClientRect()

        // Axis-Aligned Bounding Box (AABB) intersection check in viewport space
        const intersects =
          itemRect.left < boxRight &&
          itemRect.right > boxLeft &&
          itemRect.top < boxBottom &&
          itemRect.bottom > boxTop

        if (intersects) {
          newSelected.add(id)
        }
      })

      onSelectionChange(newSelected)

      // Container edge auto-scrolling logic (only if container has scrollable overflow)
      const maxScrollTop = container.scrollHeight - container.clientHeight
      if (maxScrollTop > 1) {
        const edgeThreshold = 35
        const scrollSpeed = 10

        if (e.clientY < containerRect.top + edgeThreshold && container.scrollTop > 0) {
          container.scrollTop -= scrollSpeed
        } else if (
          e.clientY > containerRect.bottom - edgeThreshold &&
          container.scrollTop < maxScrollTop - 1
        ) {
          container.scrollTop += scrollSpeed
        }
      }
    }
  }

  const handlePointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (isSelecting && containerRef.current) {
      try {
        containerRef.current.releasePointerCapture(e.pointerId)
      } catch {
        // Pointer capture might already be released
      }
    }
    setBox(null)
    setIsSelecting(false)
  }

  // Calculate position styles for selection box overlay
  let rectStyle: React.CSSProperties | null = null
  if (box && isSelecting) {
    const left = Math.min(box.startX, box.currentX)
    const top = Math.min(box.startY, box.currentY)
    const width = Math.abs(box.currentX - box.startX)
    const height = Math.abs(box.currentY - box.startY)

    rectStyle = {
      left: `${left}px`,
      top: `${top}px`,
      width: `${width}px`,
      height: `${height}px`,
    }
  }

  return (
    <div
      ref={containerRef}
      className={cn("relative overflow-auto select-none", className)}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
    >
      {children}

      {/* Render selection rectangle overlay */}
      {rectStyle && (
        <div
          style={rectStyle}
          className="absolute z-50 pointer-events-none rounded-sm border border-primary/60 bg-primary/20 shadow-xs backdrop-blur-[1px] transition-none"
        />
      )}
    </div>
  )
}
