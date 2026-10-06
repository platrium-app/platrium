import { Dialog } from "@base-ui/react/dialog"
import { FilePreviewCore } from "../filepreview/FilePreviewCore"

/** A file opened over the current page. Closed while `fileId` is null. */
export function FilePreviewDialog({
  fileId,
  onClose,
}: {
  fileId: string | null
  onClose: () => void
}) {
  return (
    <Dialog.Root open={Boolean(fileId)} onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop
          className="fixed inset-0 z-50 bg-black/15 dark:bg-black/60 backdrop-blur-xl dark:backdrop-blur-sm backdrop-saturate-200 dark:backdrop-saturate-100 transition-opacity duration-150 ease-out data-starting-style:opacity-0 data-ending-style:opacity-0"
          onContextMenu={(e) => e.stopPropagation()}
        />
        <Dialog.Popup
          initialFocus={false}
          className="fixed inset-0 z-50 flex items-center justify-center transition-opacity duration-150 data-starting-style:opacity-0 data-ending-style:opacity-0 outline-none"
          onContextMenu={(e) => e.stopPropagation()}
        >
          {fileId && <FilePreviewCore fileId={fileId} isModal onClose={onClose} />}
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
