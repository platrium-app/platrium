import { useParams } from "react-router-dom"
import { FolderOpen, AlertTriangle, FolderRoot, Folder } from "lucide-react"
import { FolderContextMenu } from "./FolderContextMenu"
import { useSetBreadcrumbs, type BreadcrumbItemType } from "@/contexts/BreadcrumbContext"
import { useQuery } from "@apollo/client/react"
import { graphql } from "@/graphql"
import { useMemo } from "react"
import { PlaceholderView } from "@/components/custom/PlaceholderView"
import { Spinner } from "@/components/ui/spinner"

const GET_FOLDER_INFO = graphql(`
  query GetFolderInfo($id: ID!) {
    item(id: $id) {
      id
      name
      path {
        id
        name
      }
    }
  }
`)

export default function FolderRootView() {
  const { id } = useParams()

  const { data, loading, error } = useQuery(GET_FOLDER_INFO, {
    variables: { id: id! },
    skip: !id,
  })

  const item = data?.item

  const breadcrumbs = useMemo<BreadcrumbItemType[]>(() => {
    const crumbs: BreadcrumbItemType[] = []

    if (loading) {
      crumbs.push({
        id: id,
        label: "Loading...",
        href: `/folder/${id}`,
        icon: Spinner,
      })
      return crumbs
    }

    if (item) {
      if (item.path && item.path.length > 0) {
        item.path.forEach((folder, index) => {
          crumbs.push({
            id: folder.id,
            label: folder.name,
            href: `/folder/${folder.id}`,
            icon: index === 0 ? FolderRoot : Folder,
          })
        })
        crumbs.push({
          id: id,
          label: item.name,
          href: `/folder/${id}`,
          icon: Folder,
        })
      } else {
        crumbs.push({
          id: id,
          label: item.name,
          href: `/folder/${id}`,
          icon: FolderRoot,
        })
      }
    } else {
      crumbs.push({
        id: id,
        label: "Invalid Resource",
        href: `/folder/${id}`,
        icon: AlertTriangle,
      })
    }

    return crumbs
  }, [id, item, loading])

  useSetBreadcrumbs(breadcrumbs)

  if (loading) {
    return (
      <div className="flex h-full min-h-[50vh] w-full flex-col items-center justify-center p-8 text-center animate-in fade-in duration-300">
        <Spinner className="size-8 text-muted-foreground" />
        <p className="mt-3 text-sm text-muted-foreground">Loading this Resource</p>
      </div>
    )
  }

  if (error) {
    return (
      <PlaceholderView
        icon={AlertTriangle}
        variant="error"
        title="Error Accessing Resource"
        description={error.message || "Failed to load folder information."}
      />
    )
  }

  if (!item) {
    return (
      <PlaceholderView
        icon={AlertTriangle}
        variant="error"
        title="Resource Not Found"
        description="The requested folder could not be found."
      />
    )
  }

  return (
    <FolderContextMenu folderId={id!}>
      <PlaceholderView
        icon={FolderOpen}
        title="This folder is empty"
        description={
          <>
            Right-click anywhere to create a new folder, or upload files directly into{" "}
            <span className="font-semibold text-foreground">{item.name}</span>.
          </>
        }
      />
    </FolderContextMenu>
  )
}





