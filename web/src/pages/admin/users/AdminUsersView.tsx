import { useState } from "react"
import { useMutation, useQuery } from "@apollo/client/react"
import { CircleAlertIcon, Plus, Search, ShieldCheck, Users } from "lucide-react"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { ConfirmModal } from "@/components/modals/ConfirmModal"
import { Button } from "@/components/ui/button"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { useSetBreadcrumbs } from "@/contexts/BreadcrumbContext"
import type { UserStatus } from "@/graphql/graphql"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { usePermissions } from "@/hooks/usePermissions"
import {
  ADMIN_IDENTITY_SOURCES,
  ADMIN_USERS,
  SET_USER_DISABLED,
} from "./adminUserQueries"
import { CreateUserDialog } from "./CreateUserDialog"
import { EditUserDialog } from "./EditUserDialog"
import { ResetPasswordDialog } from "./ResetPasswordDialog"
import type { AdminUserNode } from "./types"
import { UsersTable, type UserAction } from "./UsersTable"

const PAGE_SIZE = 50

// The "no filter" choice: a select needs a value to stand for it.
const ALL = "all"

type OpenDialog =
  | { kind: "create" }
  | { kind: "edit"; user: AdminUserNode }
  | { kind: "reset-password"; user: AdminUserNode }
  | { kind: "disable"; user: AdminUserNode }

/** Everyone in the organization, whichever identity provider they come from. */
export default function AdminUsersView() {
  useSetBreadcrumbs([
    { label: "Admin console", icon: ShieldCheck },
    { label: "Users", icon: Users },
  ])
  const { me, can, assignableRoles } = usePermissions()

  const [search, setSearch] = useState("")
  const [sourceId, setSourceId] = useState("")
  const [status, setStatus] = useState<UserStatus | "">("")
  const [dialog, setDialog] = useState<OpenDialog | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const debouncedSearch = useDebouncedValue(search.trim())
  const filtered = Boolean(debouncedSearch || sourceId || status)

  const { data, loading, error, fetchMore } = useQuery(ADMIN_USERS, {
    variables: {
      first: PAGE_SIZE,
      search: debouncedSearch || undefined,
      sourceId: sourceId || undefined,
      status: status || undefined,
    },
  })
  const { data: sourcesData } = useQuery(ADMIN_IDENTITY_SOURCES)
  const [setDisabled] = useMutation(SET_USER_DISABLED)

  const connection = data?.adminUsers
  const users = connection?.edges.map((e) => e.node) ?? []
  const sources = sourcesData?.adminIdentitySources ?? []

  const loadMore = async () => {
    if (!connection?.pageInfo.endCursor) return
    setLoadingMore(true)
    try {
      await fetchMore({ variables: { after: connection.pageInfo.endCursor } })
    } finally {
      setLoadingMore(false)
    }
  }

  const handleAction = async (action: UserAction, user: AdminUserNode) => {
    setActionError(null)
    if (action === "enable") {
      try {
        await setDisabled({ variables: { id: user.id, disabled: false } })
      } catch (err) {
        setActionError(err instanceof Error ? err.message : String(err))
      }
      return
    }
    setDialog({ kind: action, user })
  }

  const createButton = can("USERS_CREATE") ? (
    <Button onClick={() => setDialog({ kind: "create" })}>
      <Plus className="size-4" />
      <span>Create user</span>
    </Button>
  ) : null

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden">
      <div className="flex shrink-0 flex-wrap items-center gap-2 px-1 pt-1 pb-3">
        <InputGroup className="min-w-52 flex-1 sm:max-w-xs">
          <InputGroupAddon>
            <Search />
          </InputGroupAddon>
          <InputGroupInput
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or email"
            aria-label="Search users"
          />
        </InputGroup>
        <Select
          value={sourceId || ALL}
          onValueChange={(v) => setSourceId(!v || v === ALL ? "" : v)}
          items={{
            [ALL]: "All sources",
            ...Object.fromEntries(
              sources.map((s) => [s.id, s.isLocal ? "Cluster local" : s.name])
            ),
          }}
        >
          <SelectTrigger aria-label="Filter by source">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value={ALL}>All sources</SelectItem>
              {sources.map((s) => (
                <SelectItem key={s.id} value={s.id}>
                  {s.isLocal ? "Cluster local" : s.name}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        <Select
          value={status || ALL}
          onValueChange={(v) =>
            setStatus(!v || v === ALL ? "" : (v as UserStatus))
          }
          items={{
            [ALL]: "Any status",
            ACTIVE: "Active",
            DISABLED: "Disabled",
          }}
        >
          <SelectTrigger aria-label="Filter by status">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value={ALL}>Any status</SelectItem>
              <SelectItem value="ACTIVE">Active</SelectItem>
              <SelectItem value="DISABLED">Disabled</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <div className="ml-auto">{createButton}</div>
      </div>

      {actionError && (
        <p className="shrink-0 pb-2 text-sm font-medium text-destructive">
          {actionError}
        </p>
      )}

      {loading && !data ? (
        <div className="flex flex-1 items-center justify-center">
          <Spinner className="size-6 text-muted-foreground" />
        </div>
      ) : error ? (
        <PlaceholderView
          icon={CircleAlertIcon}
          variant="error"
          title="Couldn't load users"
          description={error.message}
        />
      ) : users.length === 0 ? (
        <PlaceholderView
          icon={Users}
          title={filtered ? "No users match" : "No users yet"}
          description={
            filtered
              ? "Try a different search or clear the filters."
              : "Users appear here as they are created or sign in."
          }
        />
      ) : (
        <>
          <UsersTable
            users={users}
            selfId={me?.userId}
            can={can}
            onAction={handleAction}
          />
          <div className="flex shrink-0 items-center justify-between pt-3 text-sm text-muted-foreground">
            <span>
              Showing {users.length} of {connection?.totalCount}
            </span>
            {connection?.pageInfo.hasNextPage && (
              <Button
                variant="outline"
                size="sm"
                onClick={loadMore}
                disabled={loadingMore}
              >
                {loadingMore && <Spinner className="mr-2 size-4" />}
                Load more
              </Button>
            )}
          </div>
        </>
      )}

      {dialog?.kind === "create" && (
        <CreateUserDialog
          onClose={() => setDialog(null)}
          assignableRoles={assignableRoles}
        />
      )}
      {dialog?.kind === "edit" && (
        <EditUserDialog
          user={dialog.user}
          onClose={() => setDialog(null)}
          assignableRoles={assignableRoles}
        />
      )}
      {dialog?.kind === "reset-password" && (
        <ResetPasswordDialog
          user={dialog.user}
          onClose={() => setDialog(null)}
        />
      )}
      <ConfirmModal
        isOpen={dialog?.kind === "disable"}
        onClose={() => setDialog(null)}
        title={
          dialog?.kind === "disable"
            ? `Disable ${dialog.user.displayName}?`
            : ""
        }
        description="They are signed out everywhere at once and can't sign in until you enable them again. Their files are kept."
        confirmLabel="Disable"
        destructive
        onConfirm={() =>
          dialog?.kind === "disable"
            ? setDisabled({ variables: { id: dialog.user.id, disabled: true } })
            : Promise.resolve()
        }
      />
    </div>
  )
}
