import { useState } from "react"
import { Check, Copy } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"

/** The address to register with an identity provider as an allowed redirect (callback) URL. */
export function RedirectUriBox({ uri }: { uri: string }) {
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(uri)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard access can be refused; the address is selectable on screen.
    }
  }

  return (
    <div className="flex flex-col gap-2 rounded-lg border bg-muted/40 p-3">
      <Label>Redirect (callback) URL</Label>
      <div className="flex items-center gap-2">
        <code className="min-w-0 flex-1 text-xs break-all select-all">
          {uri}
        </code>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={copy}
          aria-label="Copy redirect URL"
        >
          {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
          {copied ? "Copied" : "Copy"}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        Add this to your provider's allowed callback URLs (Auth0: Allowed
        Callback URLs).
      </p>
    </div>
  )
}
