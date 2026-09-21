import {
  Plus,
  FolderPlus,
  FileUp,
  FolderUp,
  ArrowUpDown,
  X,
  Download,
  Share2,
  Pencil,
  Trash2,
  LayoutGridIcon,
  ListIcon,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useUpload } from "@/contexts/UploadContext"
import type { SortField, SortDirection, ViewMode } from "./FolderViewTypes"
import { cn } from "@/lib/utils"

export interface FolderHeaderToolbarProps {
  selectedCount: number
  onClearSelection: () => void
  viewMode: ViewMode
  onViewModeChange: (mode: ViewMode) => void
  sortField: SortField
  sortDirection: SortDirection
  onSortChange: (field: SortField) => void
  folderId: string
}

export function FolderHeaderToolbar({
  selectedCount,
  onClearSelection,
  viewMode,
  onViewModeChange,
  sortField,
  onSortChange,
  folderId,
}: FolderHeaderToolbarProps) {
  const { triggerUpload } = useUpload()

  const sortOptions: { label: string; field: SortField }[] = [
    { label: "Name", field: "name" },
    { label: "Last Modified", field: "updatedAt" },
    { label: "File Size", field: "size" },
    { label: "Owner", field: "owner" },
  ]

  const hasSelection = selectedCount > 0

  return (
    <div className="flex h-12 w-full shrink-0 items-center justify-between pb-2 transition-colors duration-300">
      {/* LEFT SECTION: Animated morphing between Default "+ New" & Selection Bar using CSS Grid stacking */}
      <div className="grid grid-cols-1 grid-rows-1 items-center flex-1 min-w-0">
        {/* Default State: + New Button */}
        <div
          className={cn(
            "col-start-1 row-start-1 flex items-center gap-2 transition-all duration-200 ease-in-out",
            hasSelection
              ? "pointer-events-none -translate-y-2 opacity-0 invisible"
              : "translate-y-0 opacity-100 visible"
          )}
        >
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="default" />}>
              <Plus className="size-4" />
              <span>New</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-48">
              <DropdownMenuItem>
                <FolderPlus className="size-4 text-muted-foreground" />
                <span>New Folder</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => triggerUpload(folderId)}>
                <FileUp className="size-4 text-muted-foreground" />
                <span>Upload File</span>
              </DropdownMenuItem>
              <DropdownMenuItem>
                <FolderUp className="size-4 text-muted-foreground" />
                <span>Upload Folder</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {/* Selection State: Action Bar */}
        <div
          className={cn(
            "col-start-1 row-start-1 flex items-center gap-1 transition-all duration-200 ease-in-out",
            hasSelection
              ? "translate-y-0 opacity-100 visible"
              : "pointer-events-none translate-y-2 opacity-0 invisible"
          )}
        >
          <Button
            variant="ghost"
            size="icon"
            onClick={onClearSelection}
            className="h-8 w-8 text-muted-foreground hover:text-foreground shrink-0"
            title="Clear selection"
          >
            <X className="size-4" />
          </Button>

          <span className="text-xs font-medium text-muted-foreground whitespace-nowrap px-1">
            {selectedCount} selected
          </span>

          <div className="mx-1 h-4 w-px bg-border shrink-0" />

          {/* Action Buttons */}
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              disabled
              className="h-8 gap-1.5 px-1 text-xs font-medium opacity-50 cursor-not-allowed"
              title="Download"
            >
              <Download className="size-3.5" />
              <span className="hidden sm:inline">Download</span>
            </Button>

            <Button
              variant="ghost"
              size="sm"
              disabled
              className="h-8 gap-1.5 px-1 text-xs font-medium opacity-50 cursor-not-allowed"
              title="Share"
            >
              <Share2 className="size-3.5" />
              <span className="hidden sm:inline">Share</span>
            </Button>

            <Button
              variant="ghost"
              size="sm"
              disabled
              className="h-8 gap-1.5 px-1 text-xs font-medium opacity-50 cursor-not-allowed"
              title="Rename"
            >
              <Pencil className="size-3.5" />
              <span className="hidden sm:inline">Rename</span>
            </Button>

            <Button
              variant="ghost"
              size="sm"
              disabled
              className="h-8 gap-1.5 px-1 text-xs font-medium text-destructive opacity-50 cursor-not-allowed"
              title="Delete"
            >
              <Trash2 className="size-3.5" />
              <span className="hidden sm:inline">Delete</span>
            </Button>
          </div>
        </div>
      </div>

      {/* RIGHT SECTION: Sort & View Mode Toggle */}
      <div className="flex items-center gap-2 shrink-0">
        {/* Sort By Dropdown */}
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="ghost"
                className="gap-1.5 text-xs text-muted-foreground hover:text-foreground"
              />
            }
          >
            <ArrowUpDown className="size-3.5" />
            <span className="hidden sm:inline">Sort</span>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuRadioGroup
              value={sortField}
              onValueChange={(val) => onSortChange(val as SortField)}
            >
              <DropdownMenuLabel>Sort by</DropdownMenuLabel>
              {sortOptions.map((opt) => (
                <DropdownMenuRadioItem key={opt.field} value={opt.field}>
                  {opt.label}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>

        {/* View Mode Switcher */}
        <ButtonGroup>
          <Button
            variant="outline"
            size="icon"
            onClick={() => onViewModeChange("list")}
            title="List view"
            className={viewMode == "list" ? "bg-muted dark:bg-muted/40" : ""}
          >
            <ListIcon />
          </Button>
          <Button
            variant="outline"
            size="icon"
            onClick={() => onViewModeChange("grid")}
            title="Grid view"
            className={viewMode == "grid" ? "bg-muted dark:bg-muted/40" : ""}
          >
            <LayoutGridIcon />
          </Button>
        </ButtonGroup>
      </div>
    </div>
  )
}


