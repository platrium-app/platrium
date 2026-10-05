package org.platrium.platrium.ui.accounts

import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.clickable
import org.platrium.platrium.R
import org.platrium.platrium.core.account.Account
import org.platrium.platrium.core.account.AccountStatus
import org.platrium.platrium.core.account.Selection
import org.platrium.platrium.core.account.Server
import org.platrium.platrium.ui.common.Avatar

/** Switch between accounts, grouped by server, or add an account or a server. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AccountSwitcherSheet(
    servers: List<Server>,
    accounts: List<Account>,
    selection: Selection?,
    onSelect: (Account) -> Unit,
    onAddAccount: (Server) -> Unit,
    onAddServer: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState()) {
        LazyColumn(Modifier.navigationBarsPadding()) {
            item {
                Text(
                    stringResource(R.string.accounts),
                    style = MaterialTheme.typography.titleLarge,
                    modifier = Modifier.padding(horizontal = 24.dp, vertical = 8.dp),
                )
            }
            servers.forEach { server ->
                item(key = "server-${server.id}") {
                    Text(
                        server.name,
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.padding(start = 24.dp, end = 24.dp, top = 16.dp, bottom = 4.dp),
                    )
                }
                items(accounts.filter { it.serverId == server.id }, key = { it.id }) { account ->
                    val active = selection?.account?.id == account.id
                    ListItem(
                        modifier = Modifier.clickable { onSelect(account) },
                        leadingContent = { Avatar(account.email, size = 40.dp) },
                        headlineContent = { Text(account.email) },
                        supportingContent = {
                            if (account.status == AccountStatus.NEEDS_REAUTH) Text(stringResource(R.string.sign_in_again))
                        },
                        trailingContent = {
                            if (active) Icon(Icons.Outlined.Check, contentDescription = stringResource(R.string.account_active))
                        },
                    )
                }
                item(key = "add-${server.id}") {
                    ListItem(
                        modifier = Modifier.clickable { onAddAccount(server) },
                        leadingContent = { Icon(Icons.Outlined.Add, contentDescription = null) },
                        headlineContent = { Text(stringResource(R.string.add_account)) },
                    )
                }
            }
            item { HorizontalDivider(Modifier.padding(vertical = 8.dp)) }
            item {
                ListItem(
                    modifier = Modifier.clickable(onClick = onAddServer),
                    leadingContent = { Icon(Icons.Outlined.Dns, contentDescription = null) },
                    headlineContent = { Text(stringResource(R.string.add_server)) },
                )
            }
        }
    }
}
