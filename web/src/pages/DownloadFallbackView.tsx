import { useEffect, useState } from "react"
import { Loader2, ShieldAlert } from "lucide-react"
import { PlaceholderView } from "@/components/custom/PlaceholderView"

const SpinningLoader = (props: any) => (
  <Loader2 {...props} className={`animate-spin ${props.className || ""}`} />
)

export default function DownloadFallbackView() {
  const [hasError, setHasError] = useState(false)

  useEffect(() => {
    if (!('serviceWorker' in navigator)) {
      setHasError(true)
      return
    }

    // When the service worker becomes ready, trigger a reload to let it intercept the rawcontent fetch
    navigator.serviceWorker.ready.then(() => {
      setTimeout(() => {
        window.location.reload()
      }, 500)
    }).catch(() => {
      setHasError(true)
    })
  }, [])

  if (hasError) {
    return (
      <PlaceholderView
        icon={ShieldAlert}
        variant="error"
        title="Downloads Unavailable"
        description="Downloading files in the browser requires Service Workers to be enabled and running"
      />
    )
  }

  return (
    <PlaceholderView
      icon={SpinningLoader}
      variant="ghost"
      title="Setting up Platrium"
      description="Please wait while we configure the Platrium Service Worker"
    />
  )
}
