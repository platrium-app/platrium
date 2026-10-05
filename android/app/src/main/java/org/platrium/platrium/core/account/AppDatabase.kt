package org.platrium.platrium.core.account

import android.content.Context
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.Upsert
import kotlinx.coroutines.flow.Flow

class Converters {
    @TypeConverter
    fun toStatus(value: String): AccountStatus =
        runCatching { AccountStatus.valueOf(value) }.getOrDefault(AccountStatus.NEEDS_REAUTH)

    @TypeConverter
    fun fromStatus(status: AccountStatus): String = status.name
}

@Dao
interface AccountDao {
    @Query("SELECT * FROM server ORDER BY created_at, id")
    fun observeServers(): Flow<List<Server>>

    @Query("SELECT * FROM account ORDER BY created_at, id")
    fun observeAccounts(): Flow<List<Account>>

    @Query("SELECT * FROM setting")
    fun observeSettings(): Flow<List<Setting>>

    @Query("SELECT * FROM server WHERE url = :url")
    suspend fun serverByUrl(url: String): Server?

    @Query("SELECT * FROM server WHERE id = :id")
    suspend fun server(id: String): Server?

    @Upsert
    suspend fun upsert(server: Server)

    @Query("SELECT * FROM account WHERE id = :id")
    suspend fun account(id: String): Account?

    @Query(
        "SELECT * FROM account WHERE server_id = :serverId AND remote_tenant_id = :tenantId " +
            "AND remote_user_id = :userId",
    )
    suspend fun findAccount(serverId: String, tenantId: String, userId: String): Account?

    @Upsert
    suspend fun upsert(account: Account)

    @Query("UPDATE account SET status = :status WHERE id = :id")
    suspend fun setStatus(id: String, status: AccountStatus)

    @Query("SELECT value FROM setting WHERE `key` = :key")
    suspend fun setting(key: String): String?

    @Upsert
    suspend fun upsert(setting: Setting)

    @Query("DELETE FROM setting WHERE `key` = :key")
    suspend fun deleteSetting(key: String)
}

@Database(entities = [Server::class, Account::class, Setting::class], version = 1, exportSchema = true)
@TypeConverters(Converters::class)
abstract class AppDatabase : RoomDatabase() {
    abstract fun accounts(): AccountDao

    companion object {
        fun open(context: Context): AppDatabase =
            Room.databaseBuilder(context, AppDatabase::class.java, "platrium.db").build()
    }
}
