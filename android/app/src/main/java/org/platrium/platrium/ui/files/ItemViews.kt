package org.platrium.platrium.ui.files

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Download
import androidx.compose.material.icons.outlined.DriveFileRenameOutline
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.OpenInNew
import androidx.compose.material.icons.outlined.PersonAdd
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import org.platrium.platrium.R
import org.platrium.platrium.core.transfer.DownloadAction
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.ui.common.formatBytes
import org.platrium.platrium.ui.common.formatDate
import org.platrium.platrium.ui.common.iconFor

/** What can be done to an item; each action is offered only when the server's capabilities allow it. */
class ItemActions(
    val onOpen: (DriveItem) -> Unit,
    val onDownload: (DriveItem, DownloadAction) -> Unit,
    val onShare: ((DriveItem) -> Unit)?,
    val onRename: ((DriveItem) -> Unit)? = null,
    val onDelete: ((DriveItem) -> Unit)? = null,
)

private fun DriveItem.subtitle(): String = buildString {
    if (!isFolder && size != null) append(formatBytes(size)).append(" · ")
    append(formatDate(updatedAt))
}

@Composable
fun ItemRow(item: DriveItem, actions: ItemActions, modifier: Modifier = Modifier) {
    ListItem(
        modifier = modifier.clickable { actions.onOpen(item) },
        leadingContent = { Icon(iconFor(item.isFolder, item.mimeType), contentDescription = null, tint = MaterialTheme.colorScheme.primary) },
        headlineContent = { Text(item.name, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        supportingContent = { Text(item.subtitle(), maxLines = 1) },
        trailingContent = { ItemMenu(item, actions) },
    )
}

@Composable
fun ItemCard(item: DriveItem, actions: ItemActions, modifier: Modifier = Modifier) {
    Card(modifier.fillMaxWidth().clickable { actions.onOpen(item) }) {
        Column(Modifier.padding(start = 16.dp, top = 16.dp, bottom = 8.dp)) {
            Icon(iconFor(item.isFolder, item.mimeType), null, Modifier.size(36.dp), tint = MaterialTheme.colorScheme.primary)
            androidx.compose.foundation.layout.Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f).padding(top = 12.dp)) {
                    Text(item.name, style = MaterialTheme.typography.titleSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    Text(
                        item.subtitle(),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                    )
                }
                ItemMenu(item, actions)
            }
        }
    }
}

@Composable
private fun ItemMenu(item: DriveItem, actions: ItemActions) {
    var open by remember { mutableStateOf(false) }
    val canDownload = !item.isFolder && item.can("DOWNLOAD")
    val canShare = actions.onShare != null && item.can("SHARE")
    val canRename = actions.onRename != null && item.can("EDIT")
    val canDelete = actions.onDelete != null && (item.can("TRASH") || item.can("DELETE"))
    if (!(canDownload || canShare || canRename || canDelete)) return

    IconButton(onClick = { open = true }) {
        Icon(Icons.Outlined.MoreVert, contentDescription = stringResource(R.string.more_options))
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            if (canDownload) {
                MenuItem(R.string.open, Icons.Outlined.OpenInNew) { open = false; actions.onDownload(item, DownloadAction.OPEN) }
                MenuItem(R.string.download, Icons.Outlined.Download) { open = false; actions.onDownload(item, DownloadAction.SAVE) }
            }
            if (canShare) MenuItem(R.string.share, Icons.Outlined.PersonAdd) { open = false; actions.onShare?.invoke(item) }
            if (canRename) MenuItem(R.string.rename, Icons.Outlined.DriveFileRenameOutline) { open = false; actions.onRename?.invoke(item) }
            if (canDelete) MenuItem(R.string.delete, Icons.Outlined.Delete) { open = false; actions.onDelete?.invoke(item) }
        }
    }
}

@Composable
private fun MenuItem(label: Int, icon: androidx.compose.ui.graphics.vector.ImageVector, onClick: () -> Unit) {
    DropdownMenuItem(text = { Text(stringResource(label)) }, leadingIcon = { Icon(icon, null) }, onClick = onClick)
}

/** A one-field dialog for naming a new folder or renaming an item. */
@Composable
fun NameDialog(title: String, initial: String, confirmLabel: String, onConfirm: (String) -> Unit, onDismiss: () -> Unit) {
    var name by remember { mutableStateOf(initial) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            OutlinedTextField(name, { name = it }, singleLine = true, label = { Text(stringResource(R.string.name)) })
        },
        confirmButton = {
            TextButton(onClick = { onConfirm(name.trim()) }, enabled = name.isNotBlank()) { Text(confirmLabel) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
fun ConfirmDeleteDialog(item: DriveItem, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.delete_title, item.name)) },
        text = { Text(stringResource(R.string.delete_body)) },
        confirmButton = { TextButton(onClick = onConfirm) { Text(stringResource(R.string.delete)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}
