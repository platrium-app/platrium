package org.platrium.platrium.ui.setup

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Cloud
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Card
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import org.platrium.platrium.R
import org.platrium.platrium.ui.common.appViewModel

/**
 * Connect to a server, then sign in. Shown on first launch, and from the
 * account switcher to add a server (or, with [serverId], another account on one).
 * [onClose] is set when the screen is an overlay the user can leave.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SetupScreen(serverId: String?, onClose: (() -> Unit)?, onSignedIn: () -> Unit, onChangeServer: (() -> Unit)? = null) {
    val vm = appViewModel(key = "setup-${serverId.orEmpty()}") { SetupViewModel(it.accounts, serverId) }
    val state by vm.state.collectAsStateWithLifecycle()

    Scaffold(
        topBar = {
            TopAppBar(
                title = {},
                navigationIcon = {
                    if (onClose != null) {
                        IconButton(onClick = onClose) {
                            Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.back))
                        }
                    }
                },
            )
        },
    ) { padding ->
        Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.TopCenter) {
            Column(
                Modifier
                    .widthIn(max = 480.dp)
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = 24.dp, vertical = 16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Spacer(Modifier.height(24.dp))
                val server = state.server
                if (server == null) {
                    AddressStep(state, vm::onAddressChange, vm::checkServer)
                } else {
                    SignInStep(state, server.url, vm::cancelSignIn, onChangeServer ?: vm::useDifferentServer, canChangeServer = serverId == null || onChangeServer != null) {
                        vm.signIn(onSignedIn)
                    }
                }
            }
        }
    }
}

@Composable
private fun AddressStep(state: SetupUiState, onChange: (String) -> Unit, onContinue: () -> Unit) {
    Header(Icons.Outlined.Cloud, stringResource(R.string.setup_title), stringResource(R.string.setup_subtitle))

    val focus = remember { FocusRequester() }
    OutlinedTextField(
        value = state.address,
        onValueChange = onChange,
        modifier = Modifier.fillMaxWidth().focusRequester(focus),
        label = { Text(stringResource(R.string.setup_server_address)) },
        placeholder = { Text(stringResource(R.string.setup_server_address_hint)) },
        singleLine = true,
        enabled = !state.checking,
        isError = state.error != null,
        supportingText = state.error?.let { { Text(it) } },
        keyboardOptions = KeyboardOptions(
            keyboardType = KeyboardType.Uri,
            imeAction = ImeAction.Go,
            autoCorrectEnabled = false,
        ),
        keyboardActions = KeyboardActions(onGo = { if (state.address.isNotBlank()) onContinue() }),
    )
    Button(
        onClick = onContinue,
        enabled = state.address.isNotBlank() && !state.checking,
        modifier = Modifier.fillMaxWidth(),
    ) {
        if (state.checking) {
            CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
        } else {
            Text(stringResource(R.string.setup_continue))
        }
    }
}

@Composable
private fun SignInStep(
    state: SetupUiState,
    url: String,
    onCancel: () -> Unit,
    onChangeServer: () -> Unit,
    canChangeServer: Boolean,
    onSignIn: () -> Unit,
) {
    val host = url.substringAfter("://")
    Header(Icons.Outlined.Dns, stringResource(R.string.setup_sign_in_title), stringResource(R.string.setup_sign_in_subtitle, host))

    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(host, style = MaterialTheme.typography.titleMedium)
            state.details?.let {
                Text(
                    stringResource(R.string.setup_server_details, it.edition, it.version),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }

    state.error?.let {
        Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center)
    }

    Button(onClick = onSignIn, enabled = !state.signingIn, modifier = Modifier.fillMaxWidth()) {
        if (state.signingIn) {
            CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
            Spacer(Modifier.size(12.dp))
            Text(stringResource(R.string.setup_waiting))
        } else {
            Text(stringResource(R.string.setup_sign_in))
        }
    }
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
        if (state.signingIn) {
            TextButton(onClick = onCancel) { Text(stringResource(R.string.setup_cancel)) }
        } else if (canChangeServer) {
            TextButton(onClick = onChangeServer) { Text(stringResource(R.string.setup_use_different_server)) }
        }
    }
}

@Composable
private fun Header(icon: androidx.compose.ui.graphics.vector.ImageVector, title: String, subtitle: String) {
    Icon(icon, contentDescription = null, modifier = Modifier.size(64.dp), tint = MaterialTheme.colorScheme.primary)
    Text(title, style = MaterialTheme.typography.headlineMedium, textAlign = TextAlign.Center)
    Text(
        subtitle,
        style = MaterialTheme.typography.bodyLarge,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center,
    )
}
