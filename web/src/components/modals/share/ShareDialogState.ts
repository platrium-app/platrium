import type { Subject } from "./shareUtils"

/**
 * What the dialog is doing besides showing the item. Exactly one thing at a
 * time: loading is derived from the queries, and every write holds the dialog
 * in a single busy state, so controls disable from one place.
 */
export type Busy =
  | { kind: "idle" }
  | { kind: "sharing" }
  | { kind: "grant"; id: string }
  | { kind: "general" }
  | { kind: "inheritance" }

export interface DialogState {
  recipients: Subject[]
  /** Chosen role for the people being added; null until picked, then the first offered role. */
  role: string | null
  expiry: string
  busy: Busy
  error: string | null
  copied: boolean
}

export const initialDialogState: DialogState = {
  recipients: [],
  role: null,
  expiry: "",
  busy: { kind: "idle" },
  error: null,
  copied: false,
}

export type DialogAction =
  | { type: "recipients"; recipients: Subject[] }
  | { type: "role"; role: string }
  | { type: "expiry"; expiry: string }
  | { type: "start"; busy: Exclude<Busy, { kind: "idle" }> }
  | { type: "done"; error?: string | null; keep?: Subject[] }
  | { type: "error"; message: string }
  | { type: "copied"; copied: boolean }

export function dialogReducer(state: DialogState, action: DialogAction): DialogState {
  switch (action.type) {
    case "recipients":
      return { ...state, recipients: action.recipients }
    case "role":
      return { ...state, role: action.role }
    case "expiry":
      return { ...state, expiry: action.expiry }
    case "start":
      return { ...state, busy: action.busy, error: null }
    case "done": {
      // After adding people, keep only the ones that did not go through.
      const sharedPeople = state.busy.kind === "sharing"
      return {
        ...state,
        busy: { kind: "idle" },
        error: action.error ?? null,
        recipients: sharedPeople ? (action.keep ?? []) : state.recipients,
        expiry: sharedPeople && !action.error ? "" : state.expiry,
      }
    }
    case "error":
      return { ...state, error: action.message }
    case "copied":
      return { ...state, copied: action.copied }
  }
}

export type Phase = "loading" | "denied" | "ready"

/** Which screen to show, from what the queries have returned so far. */
export function phaseOf(q: {
  loading: boolean
  hasData: boolean
  denied: boolean
}): Phase {
  if (q.loading && !q.hasData) return "loading"
  if (q.denied || !q.hasData) return "denied"
  return "ready"
}
