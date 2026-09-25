import { Loader2, ShieldAlert } from "lucide-react"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { useServiceWorkerReady } from "@/hooks/useServiceWorkerReady"

const SpinningLoader = (props: any) => (
  <Loader2 {...props} className={`animate-spin ${props.className || ""}`} />
)

export default function DownloadFallbackView() {
  const { hasError } = useServiceWorkerReady(true)

  if (hasError) {
    return (
      <div className="flex h-screen w-screen items-center justify-center bg-background relative">
        <PlaceholderView
          icon={ShieldAlert}
          variant="error"
          title="Downloads Unavailable"
          description="Downloading files in the browser requires Service Workers to be enabled and running"
        />
      </div>
    )
  }

  return (
    <div className="flex h-screen w-screen items-center justify-center bg-background relative">
      <PlaceholderView
        icon={SpinningLoader}
        variant="ghost"
        title="Setting up Platrium"
        description="Please wait while we configure the Platrium Service Worker"
      />
    </div>
  )
}
