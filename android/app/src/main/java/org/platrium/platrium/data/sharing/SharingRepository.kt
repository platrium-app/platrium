package org.platrium.platrium.data.sharing

import com.apollographql.apollo.ApolloClient
import com.apollographql.apollo.api.Optional
import org.platrium.platrium.data.files.dataOrFail
import org.platrium.platrium.graphql.GeneralAccessOptionsQuery
import org.platrium.platrium.graphql.ItemAccessQuery
import org.platrium.platrium.graphql.RevokeAccessMutation
import org.platrium.platrium.graphql.SearchDirectoryQuery
import org.platrium.platrium.graphql.SetGeneralAccessMutation
import org.platrium.platrium.graphql.SetInheritanceMutation
import org.platrium.platrium.graphql.ShareItemMutation
import org.platrium.platrium.graphql.ShareRolesQuery
import org.platrium.platrium.graphql.type.GeneralAccessInput
import org.platrium.platrium.graphql.type.ShareInput

// Roles, levels and subject types are plain strings: show ones this client
// doesn't know by name, never fail on them.

data class Subject(
    val type: String,
    val id: String,
    val name: String,
    val email: String? = null,
    val isYou: Boolean = false,
)

data class Grant(
    val id: String,
    val subjectType: String,
    val subjectId: String,
    val subjectName: String,
    val role: String,
    val noDownload: Boolean,
    val expiresAt: String?,
    val isYou: Boolean,
    val inheritedFromName: String? = null,
)

data class GeneralAccess(val level: String, val role: String?, val noDownload: Boolean, val expiresAt: String?)

data class RoleOption(
    val role: String,
    val label: String,
    val description: String,
    val downloadOptional: Boolean,
)

data class AccessLevelOption(
    val level: String,
    val label: String,
    val blurb: String,
    val roles: List<String>,
    val supportsExpiry: Boolean,
)

data class ItemAccess(
    val owner: Subject,
    val inheritsPermissions: Boolean,
    val generalAccess: GeneralAccess,
    val grants: List<Grant>,
    val inherited: List<Grant>,
)

class SharingRepository(private val apollo: ApolloClient) {
    suspend fun itemAccess(itemId: String): ItemAccess {
        val a = apollo.query(ItemAccessQuery(itemId)).execute().dataOrFail().itemAccess
        fun grant(f: org.platrium.platrium.graphql.fragment.GrantFields, from: String? = null) = Grant(
            id = f.id,
            subjectType = f.subjectType,
            subjectId = f.subjectId,
            subjectName = f.subjectName,
            role = f.role,
            noDownload = f.noDownload,
            expiresAt = f.expiresAt,
            isYou = f.isYou,
            inheritedFromName = from,
        )
        return ItemAccess(
            owner = Subject(a.owner.type, a.owner.id, a.owner.name, a.owner.email, a.owner.isYou),
            inheritsPermissions = a.inheritsPermissions,
            generalAccess = GeneralAccess(
                a.generalAccess.level,
                a.generalAccess.role,
                a.generalAccess.noDownload,
                a.generalAccess.expiresAt,
            ),
            grants = a.grants.map { grant(it.grantFields) },
            inherited = a.inherited.map { grant(it.grantFields, it.inheritedFrom?.name) },
        )
    }

    suspend fun roles(itemId: String): List<RoleOption> =
        apollo.query(ShareRolesQuery(itemId)).execute().dataOrFail().shareRoles.map {
            RoleOption(it.role, it.label, it.description, it.downloadOptional)
        }

    suspend fun accessLevels(itemId: String): List<AccessLevelOption> =
        apollo.query(GeneralAccessOptionsQuery(itemId)).execute().dataOrFail().generalAccessOptions.map {
            AccessLevelOption(it.level, it.label, it.blurb, it.roles, it.supportsExpiry)
        }

    suspend fun searchDirectory(query: String, excludeAccessToItemId: String): List<Subject> =
        apollo.query(
            SearchDirectoryQuery(query, Optional.present(8), Optional.present(excludeAccessToItemId)),
        ).execute().dataOrFail().searchDirectory.map { Subject(it.type, it.id, it.name, it.email) }

    suspend fun share(
        itemId: String,
        subjectType: String,
        subjectId: String,
        role: String,
        noDownload: Boolean = false,
        expiresAt: String? = null,
    ) {
        apollo.mutation(
            ShareItemMutation(
                ShareInput(
                    itemId = itemId,
                    subjectType = subjectType,
                    subjectId = subjectId,
                    role = role,
                    noDownload = Optional.present(noDownload),
                    expiresAt = Optional.presentIfNotNull(expiresAt),
                ),
            ),
        ).execute().dataOrFail()
    }

    suspend fun revoke(grantId: String) {
        apollo.mutation(RevokeAccessMutation(grantId)).execute().dataOrFail()
    }

    suspend fun setGeneralAccess(itemId: String, level: String, role: String?, noDownload: Boolean, expiresAt: String?) {
        apollo.mutation(
            SetGeneralAccessMutation(
                GeneralAccessInput(
                    itemId = itemId,
                    level = level,
                    role = Optional.presentIfNotNull(role),
                    noDownload = Optional.present(noDownload),
                    expiresAt = Optional.presentIfNotNull(expiresAt),
                ),
            ),
        ).execute().dataOrFail()
    }

    suspend fun setInheritance(itemId: String, inherit: Boolean) {
        apollo.mutation(SetInheritanceMutation(itemId, inherit)).execute().dataOrFail()
    }
}
