import { NativeSelect } from "@/components/ui/native-select"
import { humanizeRole } from "./shareUtils"

export interface RoleChoice {
  role: string
  label: string
  description?: string
}

interface RoleSelectProps {
  value: string
  choices: RoleChoice[]
  onChange: (role: string) => void
  disabled?: boolean
  label: string
  className?: string
}

/**
 * Pick a role from the choices the server offered. A role it did not list (an
 * older grant, or one from a newer server) still shows, so the control never
 * misreports what someone has.
 */
export function RoleSelect({
  value,
  choices,
  onChange,
  disabled,
  label,
  className,
}: RoleSelectProps) {
  const known = choices.some((c) => c.role === value)
  return (
    <NativeSelect
      aria-label={label}
      value={value}
      disabled={disabled}
      className={className}
      onChange={(e) => onChange(e.target.value)}
    >
      {!known && <option value={value}>{humanizeRole(value)}</option>}
      {choices.map((c) => (
        <option key={c.role} value={c.role} title={c.description}>
          {c.label}
        </option>
      ))}
    </NativeSelect>
  )
}
