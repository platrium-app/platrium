package org.platrium.platrium.ui.transfers

import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.ui.Alignment
import org.platrium.platrium.R
import org.platrium.platrium.core.transfer.Transfer
import org.platrium.platrium.core.transfer.TransferState
import org.platrium.platrium.ui.common.formatBytes

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TransfersSheet(
    transfers: List<Transfer>,
    onCancel: (Transfer) -> Unit,
    onDismissTransfer: (Transfer) -> Unit,
    onClearFinished: () -> Unit,
    onClose: () -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onClose, sheetState = rememberModalBottomSheetState()) {
        Column(Modifier.navigationBarsPadding()) {
            Row(
                Modifier.fillMaxWidth().padding(start = 24.dp, end = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(stringResource(R.string.transfers), style = MaterialTheme.typography.titleLarge)
                if (transfers.any { !it.state.isActive }) {
                    TextButton(onClick = onClearFinished) { Text(stringResource(R.string.transfer_clear)) }
                }
            }
            if (transfers.isEmpty()) {
                Text(
                    stringResource(R.string.transfers_empty),
                    modifier = Modifier.padding(24.dp),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                LazyColumn {
                    items(transfers, key = { it.id }) { TransferRow(it, onCancel, onDismissTransfer) }
                }
            }
        }
    }
}

@Composable
private fun TransferRow(transfer: Transfer, onCancel: (Transfer) -> Unit, onDismiss: (Transfer) -> Unit) {
    ListItem(
        headlineContent = { Text(transfer.name, maxLines = 1) },
        supportingContent = {
            Column {
                val status = when (val s = transfer.state) {
                    TransferState.Preparing -> stringResource(R.string.transfer_preparing)
                    TransferState.Transferring ->
                        if (transfer.totalBytes > 0) "${formatBytes(transfer.bytesTransferred)} of ${formatBytes(transfer.totalBytes)}" else ""
                    TransferState.Completed -> stringResource(R.string.transfer_completed)
                    TransferState.Cancelled -> stringResource(R.string.transfer_cancelled)
                    is TransferState.Failed -> stringResource(R.string.transfer_failed, s.message)
                }
                Text(status)
                if (transfer.state.isActive) {
                    val progress = transfer.progress
                    if (progress != null) {
                        LinearProgressIndicator(progress = { progress }, modifier = Modifier.fillMaxWidth().padding(top = 8.dp))
                    } else {
                        LinearProgressIndicator(modifier = Modifier.fillMaxWidth().padding(top = 8.dp))
                    }
                }
            }
        },
        trailingContent = {
            if (transfer.state.isActive) {
                IconButton(onClick = { onCancel(transfer) }) {
                    Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.transfer_cancel))
                }
            } else {
                IconButton(onClick = { onDismiss(transfer) }) {
                    Icon(Icons.Outlined.Delete, contentDescription = stringResource(R.string.transfer_clear))
                }
            }
        },
    )
}
