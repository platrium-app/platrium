package org.platrium.platrium.ui.accounts

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.LockPerson
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import org.platrium.platrium.R
import org.platrium.platrium.core.account.Account
import org.platrium.platrium.core.account.Server
import org.platrium.platrium.ui.common.appViewModel
import org.platrium.platrium.ui.setup.SetupViewModel

/** The selected account's token was revoked or expired; sign in again to keep using it. */
@Composable
fun SignedOutScreen(account: Account, server: Server, onSwitchAccount: () -> Unit) {
    val vm = appViewModel(key = "signed-out-${account.id}") { SetupViewModel(it.accounts, server.id) }
    val state by vm.state.collectAsStateWithLifecycle()

    Column(
        Modifier.fillMaxSize().padding(32.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(Icons.Outlined.LockPerson, null, Modifier.size(56.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(stringResource(R.string.signed_out_title), style = MaterialTheme.typography.headlineSmall)
        Text(
            stringResource(R.string.signed_out_body, server.name, account.email),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
        state.error?.let { Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center) }
        Button(onClick = { vm.signIn {} }, enabled = !state.signingIn) {
            if (state.signingIn) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
            } else {
                Text(stringResource(R.string.setup_sign_in))
            }
        }
        if (state.signingIn) {
            TextButton(onClick = vm::cancelSignIn) { Text(stringResource(R.string.setup_cancel)) }
        } else {
            TextButton(onClick = onSwitchAccount) { Text(stringResource(R.string.accounts)) }
        }
    }
}
