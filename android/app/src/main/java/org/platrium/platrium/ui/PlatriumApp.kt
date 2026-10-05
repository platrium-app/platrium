package org.platrium.platrium.ui

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.launch
import androidx.compose.runtime.rememberCoroutineScope
import org.platrium.platrium.core.account.AccountStatus
import org.platrium.platrium.ui.accounts.AccountSwitcherSheet
import org.platrium.platrium.ui.accounts.SignedOutScreen
import org.platrium.platrium.ui.common.LoadingBox
import org.platrium.platrium.ui.common.rememberContainer
import org.platrium.platrium.ui.navigation.SignedInApp
import org.platrium.platrium.ui.setup.SetupScreen

/** Where the "add" overlay should start; saved so it survives rotation. */
private const val ADD_SERVER = ""

@Composable
fun PlatriumApp() {
    val store = rememberContainer().accounts
    val scope = rememberCoroutineScope()

    val servers by store.servers.collectAsStateWithLifecycle()
    val accounts by store.accounts.collectAsStateWithLifecycle()
    val selection by store.selection.collectAsStateWithLifecycle()
    val session by store.session.collectAsStateWithLifecycle()

    var showSwitcher by rememberSaveable { mutableStateOf(false) }
    // null: not adding; ADD_SERVER: adding a server; otherwise the id of the server to add an account to.
    var adding by rememberSaveable { mutableStateOf<String?>(null) }

    Surface(Modifier.fillMaxSize()) {
        val knownServers = servers
        val current = selection
        when {
            knownServers == null || current == null -> LoadingBox()

            adding != null -> SetupScreen(
                serverId = adding?.takeIf { it != ADD_SERVER },
                onClose = { adding = null },
                onSignedIn = { adding = null; showSwitcher = false },
            )

            // First run, or a server with no account yet.
            knownServers.isEmpty() || current.account == null -> SetupScreen(
                serverId = current.server?.id,
                onClose = null,
                onSignedIn = {},
                onChangeServer = { adding = ADD_SERVER },
            )

            current.account.status == AccountStatus.NEEDS_REAUTH || session == null ->
                current.server?.let { server ->
                    SignedOutScreen(current.account, server, onSwitchAccount = { showSwitcher = true })
                }

            else -> SignedInApp(session = session!!, openAccounts = { showSwitcher = true })
        }

        if (showSwitcher && adding == null && servers != null) {
            AccountSwitcherSheet(
                servers = servers.orEmpty(),
                accounts = accounts,
                selection = selection,
                onSelect = { scope.launch { store.select(it.id) }; showSwitcher = false },
                onAddAccount = { adding = it.id },
                onAddServer = { adding = ADD_SERVER },
                onDismiss = { showSwitcher = false },
            )
        }
    }
}
