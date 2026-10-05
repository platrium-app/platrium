package org.platrium.platrium.ui.transfers

import android.Manifest
import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.core.content.ContextCompat
import org.platrium.platrium.R
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.core.transfer.DownloadAction
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.ui.common.rememberContainer

/**
 * Starts downloads, asks once for the notification permission the progress
 * notification needs, and acts on each finished download: open it, share it,
 * or tell the user where it was saved.
 */
@Composable
fun DownloadHost(session: AccountSession, snackbar: SnackbarHostState): (DriveItem, DownloadAction) -> Unit {
    val context = LocalContext.current
    val downloads = rememberContainer().downloads
    val currentSession = rememberUpdatedState(session)
    val savedMessage = stringResource(R.string.saved_to_downloads)
    val openLabel = stringResource(R.string.open)
    val noApp = stringResource(R.string.no_app_to_open)
    val openWith = stringResource(R.string.open_with)
    val shareVia = stringResource(R.string.share_via)

    val requestNotifications = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {}
    val asked = remember { booleanArrayOf(false) }

    LaunchedEffect(Unit) {
        downloads.completed.collect { transfer ->
            val uri = transfer.uri ?: return@collect
            val mime = transfer.mimeType ?: "*/*"
            fun launch(intent: Intent) = try {
                context.startActivity(intent)
            } catch (_: ActivityNotFoundException) {
                // reported below
                throw ActivityNotFoundException()
            }
            try {
                when (transfer.action) {
                    DownloadAction.OPEN -> launch(
                        Intent.createChooser(
                            Intent(Intent.ACTION_VIEW).setDataAndType(uri, mime).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION),
                            openWith,
                        ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
                    )
                    DownloadAction.SHARE -> launch(
                        Intent.createChooser(
                            Intent(Intent.ACTION_SEND).setType(mime).putExtra(Intent.EXTRA_STREAM, uri)
                                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION),
                            shareVia,
                        ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
                    )
                    DownloadAction.SAVE -> {
                        val result = snackbar.showSnackbar(savedMessage, openLabel, duration = SnackbarDuration.Long)
                        if (result == SnackbarResult.ActionPerformed) {
                            launch(Intent(Intent.ACTION_VIEW).setDataAndType(uri, mime).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK))
                        }
                    }
                }
            } catch (_: ActivityNotFoundException) {
                snackbar.showSnackbar(noApp)
            }
        }
    }

    return { item, action ->
        if (!asked[0] &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            asked[0] = true
            requestNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
        downloads.download(currentSession.value, item.id, item.name, action)
    }
}
