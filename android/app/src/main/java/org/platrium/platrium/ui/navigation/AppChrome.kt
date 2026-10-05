package org.platrium.platrium.ui.navigation

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Download
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.res.stringResource
import org.platrium.platrium.R
import org.platrium.platrium.core.transfer.DownloadAction
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.ui.common.Avatar

/** What every screen inside the signed-in app can ask of the shell around it. */
class AppChrome(
    val email: String,
    val activeTransfers: Int,
    val snackbar: SnackbarHostState,
    val openAccounts: () -> Unit,
    val openTransfers: () -> Unit,
    val download: (DriveItem, DownloadAction) -> Unit,
    val share: (DriveItem) -> Unit,
)

val LocalAppChrome = staticCompositionLocalOf<AppChrome> { error("AppChrome not provided") }

/** The top-bar actions shared by every screen: downloads, and the account switcher. */
@Composable
fun AppBarActions() {
    val chrome = LocalAppChrome.current
    if (chrome.activeTransfers > 0) {
        IconButton(onClick = chrome.openTransfers) {
            BadgedBox(badge = { Badge() }) {
                Icon(Icons.Outlined.Download, contentDescription = stringResource(R.string.transfers))
            }
        }
    }
    IconButton(onClick = chrome.openAccounts) {
        Avatar(chrome.email)
    }
}
