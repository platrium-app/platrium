import { KeyRound, Network } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import type { AdminUserNode } from "./types"

/**
 * Where a user comes from: Platrium itself, or the external provider that
 * manages them. In a table column the name shrinks (and truncates) to fit; the
 * type is in the tooltip, and shown inline only where there is room.
 */
export function SourceBadge({
  source,
  showType = false,
}: {
  source: AdminUserNode["source"]
  showType?: boolean
}) {
  if (source.isLocal) {
    return (
      <Badge variant="outline" className="max-w-full">
        <KeyRound />
        <span className="truncate">Cluster local</span>
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="max-w-full"
      title={`${source.name} (${source.type})`}
    >
      <Network />
      <span className="truncate">{source.name}</span>
      {showType && <span className="text-muted-foreground">{source.type}</span>}
    </Badge>
  )
}
