package org.platrium.platrium.core.account

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.PrimaryKey
import java.util.UUID

enum class AccountStatus { ACTIVE, NEEDS_REAUTH }

/** A Platrium server the user has connected to. */
@Entity(tableName = "server", indices = [Index(value = ["url"], unique = true)])
data class Server(
    @PrimaryKey val id: String = UUID.randomUUID().toString(),
    val name: String,
    /** Normalized: no trailing slash, no path. */
    val url: String,
    @ColumnInfo(name = "created_at") val createdAt: Long = System.currentTimeMillis(),
) {
    val restBaseUrl: String get() = "$url/api"
    val graphqlUrl: String get() = "$url/graphql"
}

/**
 * A user signed in on a server: (server, remote tenant, remote user). The id
 * doubles as the key of the account's token in the [org.platrium.platrium.core.auth.TokenVault].
 */
@Entity(
    tableName = "account",
    foreignKeys = [
        ForeignKey(
            entity = Server::class,
            parentColumns = ["id"],
            childColumns = ["server_id"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
    indices = [
        Index("server_id"),
        Index(value = ["server_id", "remote_tenant_id", "remote_user_id"], unique = true),
    ],
)
data class Account(
    @PrimaryKey val id: String = UUID.randomUUID().toString(),
    @ColumnInfo(name = "server_id") val serverId: String,
    @ColumnInfo(name = "remote_tenant_id") val remoteTenantId: String,
    @ColumnInfo(name = "remote_user_id") val remoteUserId: String,
    val email: String,
    @ColumnInfo(name = "token_id") val tokenId: String,
    @ColumnInfo(name = "device_id") val deviceId: String?,
    val status: AccountStatus = AccountStatus.ACTIVE,
    @ColumnInfo(name = "created_at") val createdAt: Long = System.currentTimeMillis(),
    @ColumnInfo(name = "last_used_at") val lastUsedAt: Long = System.currentTimeMillis(),
)

@Entity(tableName = "setting")
data class Setting(
    @PrimaryKey val key: String,
    val value: String,
)

/** What the UI should treat as selected. */
data class Selection(val server: Server?, val account: Account?)
