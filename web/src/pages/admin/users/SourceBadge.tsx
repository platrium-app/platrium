import { KeyRound, Network } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import type { AdminUserNode } from "./types"

/** Where a user comes from: Platrium itself, or the external provider that manages them. */
export function SourceBadge({ source }: { source: AdminUserNode["source"] }) {
  if (source.isLocal) {
    return (
      <Badge variant="outline">
        <KeyRound />
        Cluster local
      </Badge>
    )
  }
  return (
    <Badge variant="outline" title={`${source.name} (${source.type})`}>
      <Network />
      <span className="max-w-40 truncate">{source.name}</span>
      <span className="text-muted-foreground">{source.type}</span>
    </Badge>
  )
}
