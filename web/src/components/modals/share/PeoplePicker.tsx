import * as React from "react"
import { useQuery } from "@apollo/client/react"
import { Users, X } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"
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

/** Pick people and groups: type to search, Enter to add, Backspace to remove the last. */
export function PeoplePicker({
  itemId,
  selected,
  onChange,
  disabled,
}: PeoplePickerProps) {
  const [text, setText] = React.useState("")
  const [open, setOpen] = React.useState(false)
  const [active, setActive] = React.useState(0)
  const inputRef = React.useRef<HTMLInputElement>(null)
  const listId = React.useId()

  const query = useDebounced(text.trim(), DEBOUNCE_MS)
  const searching = query.length >= MIN_QUERY

  const { data, loading } = useQuery(SEARCH_DIRECTORY, {
    variables: { query, first: 8, exclude: itemId },
    skip: !searching,
    fetchPolicy: "network-only",
  })

  const taken = React.useMemo(
    () => new Set(selected.map((s) => `${s.type}:${s.id}`)),
    [selected]
  )
  const results = React.useMemo(
    () =>
      searching
        ? (data?.searchDirectory ?? []).filter(
            (r) => !taken.has(`${r.type}:${r.id}`)
          )
        : [],
    [searching, data, taken]
  )

  // Keep the highlighted row inside the list as results change.
  const activeIndex = Math.min(active, Math.max(results.length - 1, 0))

  const pick = (s: Subject) => {
    onChange([...selected, s])
    setText("")
    setOpen(false)
    inputRef.current?.focus()
  }

  const remove = (s: Subject) =>
    onChange(selected.filter((x) => !(x.type === s.type && x.id === s.id)))

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown" && results.length > 0) {
      e.preventDefault()
      setOpen(true)
      setActive((activeIndex + 1) % results.length)
    } else if (e.key === "ArrowUp" && results.length > 0) {
      e.preventDefault()
      setActive((activeIndex - 1 + results.length) % results.length)
    } else if (e.key === "Enter" && open && results[activeIndex]) {
      e.preventDefault() // do not submit the dialog
      pick(results[activeIndex])
    } else if (e.key === "Escape" && open) {
      e.stopPropagation() // close the list, not the dialog
      setOpen(false)
    } else if (e.key === "Backspace" && text === "" && selected.length > 0) {
      onChange(selected.slice(0, -1))
    }
  }

  const showList = open && searching
  const hint = loading ? "Searching…" : "No one found"

  return (
    <div className="relative">
      <div
        className={cn(
          "flex min-h-9 flex-wrap items-center gap-1.5 rounded-3xl bg-input/50 px-2 py-1 focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/30",
          disabled && "opacity-50"
        )}
        onClick={() => inputRef.current?.focus()}
      >
        {selected.map((s) => (
          <span
            key={`${s.type}:${s.id}`}
            className="inline-flex items-center gap-1 rounded-full bg-secondary py-0.5 pr-1 pl-2 text-xs"
          >
            {s.type === "GROUP" && (
              <Users className="size-3 text-muted-foreground" />
            )}
            <span className="max-w-40 truncate">{s.name}</span>
            <button
              type="button"
              className="rounded-full p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
              aria-label={`Remove ${s.name}`}
              onClick={(e) => {
                e.stopPropagation()
                remove(s)
              }}
              disabled={disabled}
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          value={text}
          onChange={(e) => {
            setText(e.target.value)
            setActive(0)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 120)}
          onKeyDown={onKeyDown}
          disabled={disabled}
          role="combobox"
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-label="Add people or groups"
          placeholder={selected.length === 0 ? "Add people or groups" : ""}
          className="min-w-32 flex-1 bg-transparent px-1 py-1 text-sm outline-none placeholder:text-muted-foreground"
        />
      </div>

      {showList && (
        <ul
          id={listId}
          role="listbox"
          className="absolute z-20 mt-1 max-h-56 w-full overflow-auto rounded-2xl bg-popover p-1 shadow-lg ring-1 ring-foreground/10"
        >
          {results.length === 0 ? (
            <li className="px-3 py-2 text-sm text-muted-foreground">{hint}</li>
          ) : (
            results.map((r, i) => (
              <li
                key={`${r.type}:${r.id}`}
                role="option"
                aria-selected={i === activeIndex}
                className={cn(
                  "flex cursor-pointer items-center gap-2 rounded-xl px-2 py-1.5",
                  i === activeIndex && "bg-muted"
                )}
                onMouseEnter={() => setActive(i)}
                onMouseDown={(e) => {
                  e.preventDefault() // keep focus in the input
                  pick(r)
                }}
              >
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
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  )
}
