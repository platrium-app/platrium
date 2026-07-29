import { useParams } from "react-router-dom"
import { FolderOpen } from "lucide-react"
import { FolderContextMenu } from "./FolderContextMenu"

export default function FolderRootView() {
  const { id } = useParams()

  return (
    <FolderContextMenu>
      <div className="flex h-full min-h-[50vh] w-full animate-in flex-col items-center justify-center p-8 text-center duration-300 fade-in">
        <div className="mb-4 flex size-20 items-center justify-center rounded-full bg-muted/50">
          <FolderOpen
            className="size-10 text-muted-foreground"
            strokeWidth={1.5}
          />
        </div>
        <h2 className="text-xl font-semibold tracking-tight">
          This folder is empty
        </h2>
        <p className="mx-auto mt-2 max-w-xs text-sm text-muted-foreground">
          Right-click anywhere to create a new folder, or upload files directly
          into <span className="font-medium text-foreground">{id}</span>.
        </p>
      </div>
    </FolderContextMenu>
  )
}
