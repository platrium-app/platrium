package org.platrium.platrium.ui.share

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.data.files.GraphQLException
import org.platrium.platrium.data.sharing.AccessLevelOption
import org.platrium.platrium.data.sharing.Grant
import org.platrium.platrium.data.sharing.ItemAccess
import org.platrium.platrium.data.sharing.RoleOption
import org.platrium.platrium.data.sharing.SharingRepository
import org.platrium.platrium.data.sharing.Subject

enum class SharePhase { Loading, Denied, Failed, Ready }

data class ShareUiState(
    val phase: SharePhase = SharePhase.Loading,
    val loadError: String? = null,
    val access: ItemAccess? = null,
    val roles: List<RoleOption> = emptyList(),
    val levels: List<AccessLevelOption> = emptyList(),

    // Adding people
    val query: String = "",
    val results: List<Subject> = emptyList(),
    val recipients: List<Subject> = emptyList(),
    /** Chosen role for new people; null until picked, then the first role offered. */
    val role: String? = null,
    val expiresAt: String? = null,

    /** A write is in flight; controls disable. */
    val busy: Boolean = false,
    val busyGrantId: String? = null,
    val error: String? = null,
) {
    val activeRole: String get() = roles.firstOrNull { it.role == role }?.role ?: roles.firstOrNull()?.role.orEmpty()
}

/**
 * One sheet for sharing anything: a file, a folder, or a shared drive (whose
 * "sharing" is its membership). The server decides which roles and levels to
 * offer and what they're called, so nothing here hard-codes either.
 */
class ShareViewModel(private val repo: SharingRepository, private val item: DriveItem) : ViewModel() {
    private val _state = MutableStateFlow(ShareUiState())
    val state: StateFlow<ShareUiState> = _state.asStateFlow()

    private var searchJob: Job? = null

    init {
        load()
    }

    fun load() {
        _state.update { it.copy(phase = SharePhase.Loading) }
        viewModelScope.launch {
            try {
                val access = repo.itemAccess(item.id)
                val roles = repo.roles(item.id)
                val levels = repo.accessLevels(item.id)
                _state.update { it.copy(phase = SharePhase.Ready, access = access, roles = roles, levels = levels) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                val denied = (e as? GraphQLException)?.code in setOf("FORBIDDEN", "NOT_FOUND")
                _state.update { it.copy(phase = if (denied) SharePhase.Denied else SharePhase.Failed, loadError = friendly(e)) }
            }
        }
    }

    // ---- Adding people -------------------------------------------------

    fun onQueryChange(query: String) {
        _state.update { it.copy(query = query) }
        searchJob?.cancel()
        if (query.isBlank()) {
            _state.update { it.copy(results = emptyList()) }
            return
        }
        searchJob = viewModelScope.launch {
            delay(250)
            try {
                val found = repo.searchDirectory(query.trim(), item.id)
                _state.update { s -> s.copy(results = found.filter { f -> s.recipients.none { it.id == f.id } }) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(error = friendly(e)) }
            }
        }
    }

    fun addRecipient(subject: Subject) = _state.update {
        it.copy(recipients = it.recipients + subject, results = it.results.filterNot { r -> r.id == subject.id }, query = "")
    }

    fun removeRecipient(subject: Subject) = _state.update { it.copy(recipients = it.recipients.filterNot { r -> r.id == subject.id }) }
    fun setRole(role: String) = _state.update { it.copy(role = role) }
    fun setExpiry(iso: String?) = _state.update { it.copy(expiresAt = iso) }

    fun share() {
        val s = _state.value
        if (s.recipients.isEmpty() || s.busy) return
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            val failed = mutableListOf<Subject>()
            var lastError: Exception? = null
            for (r in s.recipients) {
                try {
                    repo.share(item.id, r.type, r.id, s.activeRole, expiresAt = s.expiresAt)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    failed += r
                    lastError = e
                }
            }
            val access = refreshed()
            _state.update {
                it.copy(
                    busy = false,
                    access = access ?: it.access,
                    // Keep only the people it didn't work for, so they can be retried.
                    recipients = failed,
                    expiresAt = if (failed.isEmpty()) null else it.expiresAt,
                    error = lastError?.let(::friendly),
                )
            }
        }
    }

    // ---- People with access ---------------------------------------------

    fun changeRole(grant: Grant, role: String) = write(grantId = grant.id) {
        val downloadOptional = _state.value.roles.firstOrNull { it.role == role }?.downloadOptional == true
        repo.share(item.id, grant.subjectType, grant.subjectId, role, noDownload = downloadOptional && grant.noDownload, expiresAt = grant.expiresAt)
    }

    fun remove(grant: Grant) = write(grantId = grant.id) { repo.revoke(grant.id) }

    // ---- General access ---------------------------------------------------

    fun setGeneralAccess(level: String, role: String?, noDownload: Boolean, expiresAt: String?) =
        write { repo.setGeneralAccess(item.id, level, role, noDownload, expiresAt) }

    fun setInheritance(inherit: Boolean) = write { repo.setInheritance(item.id, inherit) }

    fun errorShown() = _state.update { it.copy(error = null) }

    private fun write(grantId: String? = null, block: suspend () -> Unit) {
        if (_state.value.busy) return
        _state.update { it.copy(busy = true, busyGrantId = grantId, error = null) }
        viewModelScope.launch {
            var error: String? = null
            try {
                block()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                error = friendly(e)
            }
            val access = refreshed()
            _state.update { it.copy(busy = false, busyGrantId = null, access = access ?: it.access, error = error) }
        }
    }

    private suspend fun refreshed(): ItemAccess? = try {
        repo.itemAccess(item.id)
    } catch (e: CancellationException) {
        throw e
    } catch (_: Exception) {
        null
    }

    private fun friendly(e: Exception): String {
        val code = (e as? GraphQLException)?.code
        return when (code) {
            "FORBIDDEN" ->
                if (e.message?.contains("public sharing") == true) "Your organization does not allow public sharing."
                else "You don't have permission to do that."
            "NOT_FOUND" -> "That item or person no longer exists."
            "UNAUTHENTICATED" -> "Your session has ended. Sign in again."
            "CONFLICT" -> "That already exists."
            else -> e.message ?: "Something went wrong. Try again."
        }
    }
}
