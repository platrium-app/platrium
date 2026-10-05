import { OptionMenu } from "./OptionMenu"
import { humanizeRole } from "./shareUtils"

export interface RoleChoice {
  role: string
  label: string
  description?: string
  /** A grant of this role may withhold download. */
  downloadOptional?: boolean
}

interface RoleSelectProps {
  value: string
  choices: RoleChoice[]
  onChange: (role: string) => void
  disabled?: boolean
  label: string
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
}: RoleSelectProps) {
  const known = choices.some((c) => c.role === value)
  const options = [
    ...(known ? [] : [{ value, label: humanizeRole(value) }]),
    ...choices.map((c) => ({ value: c.role, label: c.label })),
  ]
  return (
    <OptionMenu
      label={label}
      value={value}
      options={options}
      disabled={disabled}
      onChange={onChange}
    />
  )
}
