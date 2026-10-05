package org.platrium.platrium.ui.files

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.padding
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import org.platrium.platrium.R
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.data.files.Drive
import org.platrium.platrium.data.files.FilesRepository
import org.platrium.platrium.ui.common.EmptyState
import org.platrium.platrium.ui.common.LoadingBox
import org.platrium.platrium.ui.common.appViewModel
import org.platrium.platrium.ui.common.formatBytes
import org.platrium.platrium.ui.navigation.AppBarActions

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DrivesScreen(session: AccountSession, onOpenFolder: (String) -> Unit) {
    val vm = appViewModel(key = "drives-${session.account.id}-${session.account.tokenId}") {
        DrivesViewModel(FilesRepository(session.apollo))
    }
    val state by vm.state.collectAsStateWithLifecycle()

    Scaffold(
        topBar = { TopAppBar(title = { Text(stringResource(R.string.nav_files)) }, actions = { AppBarActions() }) },
    ) { padding ->
        PullToRefreshBox(state.refreshing, vm::refresh, Modifier.padding(padding).fillMaxSize()) {
            when {
                state.loading -> LoadingBox()
                state.error != null -> EmptyState(
                    Icons.Outlined.Folder, state.error.orEmpty(),
                    actionLabel = stringResource(R.string.retry), onAction = vm::refresh,
                )
                state.drives.isEmpty() -> EmptyState(Icons.Outlined.Folder, stringResource(R.string.drives_empty))
                else -> {
                    val (shared, mine) = state.drives.partition { it.isShared }
                    LazyColumn(Modifier.fillMaxSize()) {
                        if (mine.isNotEmpty()) {
                            item { SectionHeader(stringResource(R.string.drives_private)) }
                            items(mine, key = { it.id }) { DriveRow(it, onOpenFolder) }
                        }
                        if (shared.isNotEmpty()) {
                            item { SectionHeader(stringResource(R.string.drives_shared)) }
                            items(shared, key = { it.id }) { DriveRow(it, onOpenFolder) }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SectionHeader(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.labelLarge,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 16.dp, bottom = 4.dp),
    )
}

@Composable
private fun DriveRow(drive: Drive, onOpen: (String) -> Unit) {
    ListItem(
        modifier = Modifier.clickable { onOpen(drive.id) },
        leadingContent = {
            Icon(
                if (drive.isShared) org.platrium.platrium.ui.common.SharedFolderIcon else Icons.Outlined.Folder,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
            )
        },
        headlineContent = { Text(drive.name) },
        supportingContent = drive.storageUsed?.let { used ->
            {
                Text(
                    drive.storageQuota?.let { "${formatBytes(used)} of ${formatBytes(it)} used" } ?: "${formatBytes(used)} used",
                )
            }
        },
    )
}
