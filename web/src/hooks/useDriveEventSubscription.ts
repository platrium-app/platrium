import { graphql } from "@/graphql"
import { useSubscription, useApolloClient } from "@apollo/client/react"

const DRIVE_ITEM_CHANGED_SUB = graphql(`
  subscription DriveItemChanged {
    driveItemChanged {
      eventType
      itemId
      deletedId
    }
  }
`)

export function useDriveEventSubscription(onUpdateNeeded: () => void) {
  const client = useApolloClient()

  useSubscription(DRIVE_ITEM_CHANGED_SUB, {
    onData: ({ data }) => {
      const event = data.data?.driveItemChanged
      if (!event) return

      if (event.eventType === "DELETED" && event.deletedId) {
        // Surgically remove it from Apollo's normalized cache.
        // We evict both File and Folder typenames since we don't know which it is.
        client.cache.evict({ id: `File:${event.deletedId}` })
        client.cache.evict({ id: `Folder:${event.deletedId}` })
        client.cache.gc()
      } else if (event.eventType === "UPDATED" && event.itemId) {
        // An item was created, renamed, or moved.
        // The safest way to handle connections (paginated lists) is to refetch.
        // Apollo will merge the refetched list seamlessly without jumping the scroll position.
        onUpdateNeeded()
      }
    },
  })
}
