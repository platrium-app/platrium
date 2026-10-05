package org.platrium.platrium.ui.setup

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.platrium.platrium.core.account.AccountStore
import org.platrium.platrium.core.account.Server
import org.platrium.platrium.core.account.ServerUrlException
import org.platrium.platrium.core.auth.SignInException
import org.platrium.platrium.core.network.ServerDetails
import org.platrium.platrium.core.network.ServerProbeException

data class SetupUiState(
    val address: String = "",
    /** Set once the server has been checked; the screen then offers to sign in. */
    val server: Server? = null,
    val details: ServerDetails? = null,
    val checking: Boolean = false,
    val signingIn: Boolean = false,
    val error: String? = null,
)

/**
 * First run, adding a server, and adding an account on a known server are one
 * flow: pick a server (unless [serverId] already names one), then sign in.
 */
class SetupViewModel(private val store: AccountStore, serverId: String?) : ViewModel() {
    private val _state = MutableStateFlow(SetupUiState())
    val state: StateFlow<SetupUiState> = _state.asStateFlow()

    private var signInJob: Job? = null

    init {
        if (serverId != null) {
            store.servers.value?.firstOrNull { it.id == serverId }?.let { server ->
                _state.update { it.copy(server = server) }
                // Show what the server is, as when it was just added. Best effort.
                viewModelScope.launch {
                    try {
                        val details = org.platrium.platrium.core.network.ServerProbe.fetch(server.url)
                        _state.update { it.copy(details = details) }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (_: Exception) {
                    }
                }
            }
        }
    }

    fun onAddressChange(address: String) = _state.update { it.copy(address = address, error = null) }

    fun checkServer() {
        val current = _state.value
        if (current.checking) return
        _state.update { it.copy(checking = true, error = null) }
        viewModelScope.launch {
            try {
                val (server, details) = store.addServer(current.address)
                _state.update { it.copy(server = server, details = details, checking = false) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(checking = false, error = describe(e)) }
            }
        }
    }

    fun signIn(onSignedIn: () -> Unit) {
        val server = _state.value.server ?: return
        if (signInJob?.isActive == true) return
        _state.update { it.copy(signingIn = true, error = null) }
        signInJob = viewModelScope.launch {
            try {
                store.signIn(server)
                _state.update { it.copy(signingIn = false) }
                onSignedIn()
            } catch (e: CancellationException) {
                _state.update { it.copy(signingIn = false) }
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(signingIn = false, error = describe(e)) }
            }
        }
    }

    fun cancelSignIn() {
        signInJob?.cancel()
    }

    fun useDifferentServer() = _state.update { SetupUiState() }

    private fun describe(e: Exception): String = when (e) {
        is ServerUrlException, is ServerProbeException, is SignInException -> e.message.orEmpty()
        else -> e.message ?: "Something went wrong."
    }
}
