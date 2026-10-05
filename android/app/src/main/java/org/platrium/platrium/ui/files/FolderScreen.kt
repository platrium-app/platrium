package org.platrium.platrium.ui.files

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.CreateNewFolder
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.GridView
import androidx.compose.material.icons.outlined.PersonAdd
import androidx.compose.material.icons.outlined.ViewList
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import org.platrium.platrium.R
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.core.transfer.DownloadAction
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.data.files.FilesRepository
import org.platrium.platrium.ui.common.EmptyState
import org.platrium.platrium.ui.common.LoadingBox
import org.platrium.platrium.ui.common.appViewModel
import org.platrium.platrium.ui.navigation.AppBarActions
import org.platrium.platrium.ui.navigation.LocalAppChrome

private sealed interface FolderDialog {
    data object NewFolder : FolderDialog
    data class Rename(val item: DriveItem) : FolderDialog
    data class Delete(val item: DriveItem) : FolderDialog
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FolderScreen(session: AccountSession, folderId: String, onOpenFolder: (String) -> Unit, onBack: () -> Unit) {
    val vm = appViewModel(key = "folder-${session.account.id}-${session.account.tokenId}-$folderId") {
        FolderViewModel(FilesRepository(session.apollo), folderId)
    }
    val contents by vm.contents.collectAsStateWithLifecycle()
    val details by vm.details.collectAsStateWithLifecycle()
    val message by vm.message.collectAsStateWithLifecycle()
    val chrome = LocalAppChrome.current

    var grid by rememberSaveable { mutableStateOf(false) }
    var dialog by remember { mutableStateOf<FolderDialog?>(null) }

    LaunchedEffect(message) {
        message?.let { chrome.snackbar.showSnackbar(it); vm.messageShown() }
    }

    val folder = details?.item
    val actions = ItemActions(
        onOpen = { item ->
            if (item.isFolder) onOpenFolder(item.id) else chrome.download(item, DownloadAction.OPEN)
        },
        onDownload = chrome.download,
        onShare = chrome.share,
        onRename = { dialog = FolderDialog.Rename(it) },
        onDelete = { dialog = FolderDialog.Delete(it) },
    )

    val scrollBehavior = TopAppBarDefaults.pinnedScrollBehavior()
    Scaffold(
        modifier = Modifier.nestedScroll(scrollBehavior.nestedScrollConnection),
        topBar = {
            TopAppBar(
                title = { Text(folder?.name ?: "", maxLines = 1) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.back))
                    }
                },
                actions = {
                    if (folder?.can("SHARE") == true) {
                        IconButton(onClick = { chrome.share(folder) }) {
                            Icon(Icons.Outlined.PersonAdd, contentDescription = stringResource(R.string.share))
                        }
                    }
                    IconButton(onClick = { grid = !grid }) {
                        Icon(
                            if (grid) Icons.Outlined.ViewList else Icons.Outlined.GridView,
                            contentDescription = stringResource(if (grid) R.string.view_list else R.string.view_grid),
                        )
                    }
                    AppBarActions()
                },
                scrollBehavior = scrollBehavior,
            )
        },
        floatingActionButton = {
            if (folder?.can("CREATE") == true) {
                FloatingActionButton(onClick = { dialog = FolderDialog.NewFolder }) {
                    Icon(Icons.Outlined.CreateNewFolder, contentDescription = stringResource(R.string.new_folder))
                }
            }
        },
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = contents.refreshing,
            onRefresh = vm::refresh,
            modifier = Modifier.padding(padding).fillMaxSize(),
        ) {
            when {
                contents.loading -> LoadingBox()
                contents.error != null && contents.items.isEmpty() ->
                    EmptyState(
                        Icons.Outlined.FolderOpen,
                        contents.error.orEmpty(),
                        actionLabel = stringResource(R.string.retry),
                        onAction = vm::refresh,
                    )
                contents.items.isEmpty() -> EmptyState(Icons.Outlined.FolderOpen, stringResource(R.string.folder_empty))
                grid -> ItemGrid(contents.items, contents.loadingMore, actions, vm::loadMore)
                else -> ItemList(contents.items, contents.loadingMore, actions, vm::loadMore)
            }
        }
    }

    when (val d = dialog) {
        FolderDialog.NewFolder -> NameDialog(
            title = stringResource(R.string.new_folder),
            initial = "",
            confirmLabel = stringResource(R.string.create),
            onConfirm = { vm.createFolder(it); dialog = null },
            onDismiss = { dialog = null },
        )
        is FolderDialog.Rename -> NameDialog(
            title = stringResource(R.string.rename),
            initial = d.item.name,
            confirmLabel = stringResource(R.string.rename),
            onConfirm = { vm.rename(d.item, it); dialog = null },
            onDismiss = { dialog = null },
        )
        is FolderDialog.Delete -> ConfirmDeleteDialog(
            item = d.item,
            onConfirm = { vm.delete(d.item); dialog = null },
            onDismiss = { dialog = null },
        )
        null -> Unit
    }
}

@Composable
fun ItemList(items: List<DriveItem>, loadingMore: Boolean, actions: ItemActions, onLoadMore: () -> Unit) {
    val state = rememberLazyListState()
    OnNearEnd(state, items.size, onLoadMore)
    LazyColumn(Modifier.fillMaxSize(), state = state, contentPadding = PaddingValues(bottom = 88.dp)) {
        items(items, key = { it.id }) { ItemRow(it, actions) }
        if (loadingMore) item { LoadingMoreRow() }
    }
}

@Composable
fun ItemGrid(items: List<DriveItem>, loadingMore: Boolean, actions: ItemActions, onLoadMore: () -> Unit) {
    val state = rememberLazyGridState()
    OnNearEnd(state, items.size, onLoadMore)
    LazyVerticalGrid(
        columns = GridCells.Adaptive(160.dp),
        modifier = Modifier.fillMaxSize(),
        state = state,
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 88.dp),
        horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(12.dp),
        verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(12.dp),
    ) {
        items(items, key = { it.id }) { ItemCard(it, actions) }
    }
}

@Composable
private fun LoadingMoreRow() {
    Box(Modifier.fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
}

/** Calls [onLoadMore] when the user scrolls within a screenful of the last loaded item. */
@Composable
fun OnNearEnd(state: LazyListState, count: Int, onLoadMore: () -> Unit) {
    LaunchedEffect(state, count) {
        snapshotFlow { state.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .collect { if (it >= count - 8) onLoadMore() }
    }
}

@Composable
fun OnNearEnd(state: LazyGridState, count: Int, onLoadMore: () -> Unit) {
    LaunchedEffect(state, count) {
        snapshotFlow { state.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .collect { if (it >= count - 8) onLoadMore() }
    }
}
