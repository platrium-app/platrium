import { ChevronDown, type LucideIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

export interface Option {
  value: string
  label: string
  icon?: LucideIcon
}

interface OptionMenuProps {
  value: string
  options: Option[]
  onChange: (value: string) => void
  label: string
  disabled?: boolean
  /** Sized to its longest option instead of the button, for long labels. */
  wide?: boolean
}

/** Choose one option from a stock shadcn dropdown menu. */
export function OptionMenu({
  value,
  options,
  onChange,
  label,
  disabled,
  wide,
}: OptionMenuProps) {
  const current = options.find((o) => o.value === value)
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="outline" aria-label={label} disabled={disabled} />
        }
      >
        {current?.icon && <current.icon />}
        {current?.label ?? value}
        <ChevronDown />
      </DropdownMenuTrigger>
      <DropdownMenuContent className={wide ? "w-max min-w-64" : undefined}>
        <DropdownMenuRadioGroup value={value} onValueChange={onChange}>
          {options.map((o) => (
            <DropdownMenuRadioItem key={o.value} value={o.value} closeOnClick>
              {o.icon && <o.icon />}
              {o.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
