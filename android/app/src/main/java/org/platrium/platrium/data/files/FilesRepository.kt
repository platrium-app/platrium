package org.platrium.platrium.data.files

import com.apollographql.apollo.ApolloClient
import com.apollographql.apollo.api.ApolloResponse
import com.apollographql.apollo.api.Operation
import org.platrium.platrium.graphql.CreateFolderMutation
import org.platrium.platrium.graphql.DeleteItemMutation
import org.platrium.platrium.graphql.GetDrivesQuery
import org.platrium.platrium.graphql.GetFolderContentsQuery
import org.platrium.platrium.graphql.GetItemQuery
import org.platrium.platrium.graphql.RenameItemMutation
import org.platrium.platrium.graphql.SharedWithMeQuery
import org.platrium.platrium.graphql.fragment.ItemFields
import com.apollographql.apollo.api.Optional

/** A file or folder as the UI shows it. */
data class DriveItem(
    val id: String,
    val parentId: String?,
    val name: String,
    val isFolder: Boolean,
    val size: Long?,
    val mimeType: String?,
    val updatedAt: String,
    val capabilities: Set<String>,
) {
    fun can(capability: String) = capability in capabilities
}

data class Drive(val id: String, val name: String, val isShared: Boolean, val storageUsed: Long?, val storageQuota: Long?)

data class Page<T>(val items: List<T>, val endCursor: String?, val hasNextPage: Boolean)

data class Breadcrumb(val id: String, val name: String)

data class ItemDetails(val item: DriveItem, val path: List<Breadcrumb>)

data class SharedItem(val item: DriveItem, val role: String)

/** [code] is the server's stable error code (`FORBIDDEN`, `NOT_FOUND`, `UNAUTHENTICATED`, ...), when it sent one. */
class GraphQLException(message: String, val code: String? = null) : Exception(message) {
    val isUnauthorized: Boolean get() = code == "UNAUTHENTICATED"
}

internal fun ItemFields.toDriveItem() = DriveItem(
    id = id,
    parentId = parentId,
    name = name,
    isFolder = type == org.platrium.platrium.graphql.type.DriveItemType.FOLDER,
    size = onFile?.size,
    mimeType = onFile?.mimeType,
    updatedAt = updatedAt,
    capabilities = myCapabilities.toSet(),
)

/** Unwraps a response, turning transport and GraphQL errors into one exception type. */
internal fun <D : Operation.Data> ApolloResponse<D>.dataOrFail(): D {
    data?.let { if (!hasErrors()) return it }
    val message = errors?.firstOrNull()?.message ?: exception?.message ?: "Something went wrong."
    throw GraphQLException(message, errors?.firstOrNull()?.extensions?.get("code") as? String)
}

class FilesRepository(private val apollo: ApolloClient) {
    suspend fun drives(): List<Drive> =
        apollo.query(GetDrivesQuery()).execute().dataOrFail().drives.map {
            Drive(
                id = it.id,
                name = it.name,
                isShared = it.driveMetadata?.driveType == org.platrium.platrium.graphql.type.DriveType.SHARED,
                storageUsed = it.driveMetadata?.storageUsed,
                storageQuota = it.driveMetadata?.storageQuota,
            )
        }

    suspend fun folderContents(folderId: String, after: String? = null, pageSize: Int = 50): Page<DriveItem> {
        val data = apollo.query(
            GetFolderContentsQuery(folderId, Optional.present(pageSize), Optional.presentIfNotNull(after)),
        ).execute().dataOrFail().folderContents
        return Page(
            items = data.edges.map { it.node.itemFields.toDriveItem() },
            endCursor = data.pageInfo.endCursor,
            hasNextPage = data.pageInfo.hasNextPage,
        )
    }

    suspend fun item(id: String): ItemDetails? {
        val item = apollo.query(GetItemQuery(id)).execute().dataOrFail().item ?: return null
        return ItemDetails(item.itemFields.toDriveItem(), item.path.map { Breadcrumb(it.id, it.name) })
    }

    suspend fun sharedWithMe(after: String? = null): Page<SharedItem> {
        val data = apollo.query(SharedWithMeQuery(Optional.present(50), Optional.presentIfNotNull(after)))
            .execute().dataOrFail().sharedWithMe
        return Page(
            items = data.edges.map { SharedItem(it.node.item.itemFields.toDriveItem(), it.node.role) },
            endCursor = data.pageInfo.endCursor,
            hasNextPage = data.pageInfo.hasNextPage,
        )
    }

    suspend fun createFolder(parentId: String, name: String) {
        apollo.mutation(CreateFolderMutation(parentId, name)).execute().dataOrFail()
    }

    suspend fun rename(id: String, newName: String) {
        apollo.mutation(RenameItemMutation(id, newName)).execute().dataOrFail()
    }

    suspend fun delete(id: String) {
        apollo.mutation(DeleteItemMutation(id)).execute().dataOrFail()
    }
}
