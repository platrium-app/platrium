import * as React from "react"
import { useQuery } from "@apollo/client/react"
import { Users } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxItem,
  ComboboxList,
  ComboboxValue,
  useComboboxAnchor,
} from "@/components/ui/combobox"
import { SEARCH_DIRECTORY } from "./shareQueries"
import { initials, type Subject } from "./shareUtils"

const MIN_QUERY = 2
const DEBOUNCE_MS = 200

interface PeoplePickerProps {
  /** The item being shared; people who already have access are not offered again. */
  itemId: string
  selected: Subject[]
  onChange: (next: Subject[]) => void
  disabled?: boolean
}

function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = React.useState(value)
  React.useEffect(() => {
    const t = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return debounced
}

const keyOf = (s: Subject) => `${s.type}:${s.id}`

/** Pick people and groups: type to search the directory, choose to add as a chip. */
export function PeoplePicker({
  itemId,
  selected,
  onChange,
  disabled,
}: PeoplePickerProps) {
  const anchor = useComboboxAnchor()
  const [text, setText] = React.useState("")
  const [open, setOpen] = React.useState(false)

  const query = useDebounced(text.trim(), DEBOUNCE_MS)
  const searching = text.trim().length >= MIN_QUERY

  const { data, loading } = useQuery(SEARCH_DIRECTORY, {
    variables: { query, first: 8, exclude: itemId },
    skip: query.length < MIN_QUERY,
    fetchPolicy: "network-only",
  })

  const taken = React.useMemo(() => new Set(selected.map(keyOf)), [selected])
  const results = React.useMemo<Subject[]>(
    () =>
      searching
        ? (data?.searchDirectory ?? []).filter((r) => !taken.has(keyOf(r)))
        : [],
    [searching, data, taken]
  )

  return (
    <Combobox
      multiple
      autoHighlight
      filter={null} // the server does the searching
      items={results}
      value={selected}
      onValueChange={(next) => {
        onChange(next)
        setText("")
      }}
      inputValue={text}
      onInputValueChange={setText}
      open={open && searching}
      onOpenChange={setOpen}
      disabled={disabled}
      itemToStringLabel={(s: Subject) => s.name}
      itemToStringValue={keyOf}
      isItemEqualToValue={(a: Subject, b: Subject) => keyOf(a) === keyOf(b)}
    >
      <ComboboxChips ref={anchor}>
        <ComboboxValue>
          {(values: Subject[]) => (
            <>
              {values.map((s) => (
                <ComboboxChip key={keyOf(s)} aria-label={s.name}>
                  {s.type === "GROUP" && <Users className="size-3" />}
                  {s.name}
                </ComboboxChip>
              ))}
              <ComboboxChipsInput
                aria-label="Add people or groups"
                placeholder={values.length === 0 ? "Add people or groups" : ""}
              />
            </>
          )}
        </ComboboxValue>
      </ComboboxChips>
      <ComboboxContent anchor={anchor}>
        <ComboboxEmpty>{loading ? "Searching…" : "No one found"}</ComboboxEmpty>
        <ComboboxList>
          {(r: Subject) => (
            <ComboboxItem key={keyOf(r)} value={r}>
              <Avatar size="sm">
                <AvatarFallback>
                  {r.type === "GROUP" ? (
                    <Users className="size-3" />
                  ) : (
                    initials(r.name)
                  )}
                </AvatarFallback>
              </Avatar>
              <div className="min-w-0 leading-tight">
                <div className="truncate text-sm">{r.name}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {r.type === "GROUP" ? "Group" : (r.email ?? "")}
                </div>
              </div>
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
