package org.platrium.platrium.ui.files

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.FolderShared
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import org.platrium.platrium.R
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.core.transfer.DownloadAction
import org.platrium.platrium.data.files.FilesRepository
import org.platrium.platrium.ui.common.EmptyState
import org.platrium.platrium.ui.common.LoadingBox
import org.platrium.platrium.ui.common.appViewModel
import org.platrium.platrium.ui.navigation.AppBarActions
import org.platrium.platrium.ui.navigation.LocalAppChrome

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SharedWithMeScreen(session: AccountSession, onOpenFolder: (String) -> Unit) {
    val vm = appViewModel(key = "shared-${session.account.id}-${session.account.tokenId}") {
        SharedWithMeViewModel(FilesRepository(session.apollo))
    }
    val state by vm.state.collectAsStateWithLifecycle()
    val chrome = LocalAppChrome.current
    val actions = ItemActions(
        onOpen = { item -> if (item.isFolder) onOpenFolder(item.id) else chrome.download(item, DownloadAction.OPEN) },
        onDownload = chrome.download,
        onShare = chrome.share,
    )

    Scaffold(
        topBar = { TopAppBar(title = { Text(stringResource(R.string.nav_shared)) }, actions = { AppBarActions() }) },
    ) { padding ->
        PullToRefreshBox(state.refreshing, vm::refresh, Modifier.padding(padding).fillMaxSize()) {
            when {
                state.loading -> LoadingBox()
                state.error != null && state.items.isEmpty() -> EmptyState(
                    Icons.Outlined.FolderShared, state.error.orEmpty(),
                    actionLabel = stringResource(R.string.retry), onAction = vm::refresh,
                )
                state.items.isEmpty() -> EmptyState(Icons.Outlined.FolderShared, stringResource(R.string.shared_empty))
                else -> {
                    val listState = rememberLazyListState()
                    OnNearEnd(listState, state.items.size, vm::loadMore)
                    LazyColumn(Modifier.fillMaxSize(), state = listState) {
                        items(state.items, key = { it.item.id }) { ItemRow(it.item, actions) }
                    }
                }
            }
        }
    }
}
