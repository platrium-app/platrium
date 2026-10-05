// What the signed-in user (or an anonymous visitor) may do with an item. The
// server computes these and sends them as `myCapabilities`; the client only
// uses them to show or hide actions. The server enforces them regardless.
export type Capability =
  | "LIST"
  | "VIEW"
  | "DOWNLOAD"
  | "COMMENT"
  | "CREATE"
  | "EDIT"
  | "DELETE"
  | "MOVE"
  | "TRASH"
  | "MOVE_OUT"
  | "SHARE"
  | "MANAGE"
  | "DELETE_DRIVE"

type Capable = { capabilities?: readonly string[] | null }

export function hasCapability(caps: readonly string[] | null | undefined, cap: Capability): boolean {
  return !!caps && caps.includes(cap)
}

/** True when every item allows the capability. An empty selection allows nothing. */
export function allCan(items: readonly Capable[], cap: Capability): boolean {
  return items.length > 0 && items.every((i) => hasCapability(i.capabilities, cap))
}

/**
 * A drive's root that can be shared is a shared drive: the server never lets a
 * private drive be opened to others. Sharing it means managing its members.
 */
export function isSharedDriveRoot(item: { parentId?: string | null } | null | undefined): boolean {
  return !!item && !item.parentId
}
