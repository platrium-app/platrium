package org.platrium.platrium.ui.settings

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import org.platrium.platrium.R
import org.platrium.platrium.core.account.AccountStatus
import org.platrium.platrium.core.auth.DeviceDescriptor
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.ui.navigation.AppBarActions

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(session: AccountSession, device: DeviceDescriptor) {
    Scaffold(
        topBar = { TopAppBar(title = { Text(stringResource(R.string.nav_settings)) }, actions = { AppBarActions() }) },
    ) { padding ->
        Column(Modifier.padding(padding).verticalScroll(rememberScrollState())) {
            Section(R.string.settings_account) {
                Row(R.string.settings_email, session.account.email)
                Row(
                    R.string.settings_status,
                    stringResource(
                        if (session.account.status == AccountStatus.ACTIVE) R.string.settings_status_active
                        else R.string.settings_status_signed_out,
                    ),
                )
            }
            Section(R.string.settings_server) {
                Row(R.string.settings_server, session.server.url.substringAfter("://"))
            }
            Section(R.string.settings_device) {
                Row(R.string.settings_device_name, device.name)
                Row(R.string.settings_app_version, device.appVersion)
            }
        }
    }
}

@Composable
private fun Section(title: Int, content: @Composable () -> Unit) {
    Text(
        stringResource(title),
        style = MaterialTheme.typography.labelLarge,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 20.dp, bottom = 4.dp),
    )
    content()
}

@Composable
private fun Row(label: Int, value: String) {
    ListItem(
        overlineContent = { Text(stringResource(label)) },
        headlineContent = { Text(value) },
    )
}
