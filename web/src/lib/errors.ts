// The server prefixes errors with their kind ("invalid: ...", "forbidden: ...")
// so the code can tell them apart; people only need the sentence after it.
const KIND_PREFIX = /^(invalid|forbidden|conflict|not found|unauthorized): /i

/** A message fit to show in a form, from whatever a failed request threw. */
export function errorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err ?? "")
  return (
    raw.replace(KIND_PREFIX, "") || "Something went wrong. Please try again."
  )
}
