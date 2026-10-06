/** "SUPER_ADMIN" -> "Super admin". Roles are plain strings, so the label is derived, not listed. */
export function roleLabel(role: string): string {
  const words = role.toLowerCase().replace(/_/g, " ")
  return words.charAt(0).toUpperCase() + words.slice(1)
}
