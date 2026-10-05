package org.platrium.platrium.ui.navigation

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Icon
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.adaptive.navigationsuite.NavigationSuiteScaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.ui.common.rememberContainer
import org.platrium.platrium.ui.files.DrivesScreen
import org.platrium.platrium.ui.files.FolderScreen
import org.platrium.platrium.ui.files.SharedWithMeScreen
import org.platrium.platrium.ui.settings.SettingsScreen
import org.platrium.platrium.ui.share.ShareSheet
import org.platrium.platrium.ui.transfers.DownloadHost
import org.platrium.platrium.ui.transfers.TransfersSheet

/** The app once an account is selected and signed in: bottom/side navigation over files, shared items and settings. */
@Composable
fun SignedInApp(session: AccountSession, openAccounts: () -> Unit) {
    val container = rememberContainer()
    val nav = rememberNavController()
    val snackbar = remember { SnackbarHostState() }

    val transfers by container.downloads.transfers.collectAsStateWithLifecycle()
    var showTransfers by remember { mutableStateOf(false) }
    var sharing by remember { mutableStateOf<DriveItem?>(null) }
    val download = DownloadHost(session, snackbar)

    // A different account has different files: start over at the root.
    LaunchedEffect(session.account.id) {
        nav.navigate(Routes.FILES) { popUpTo(nav.graph.findStartDestination().id) { inclusive = false } }
    }

    val chrome = AppChrome(
        email = session.account.email,
        activeTransfers = transfers.count { it.state.isActive },
        snackbar = snackbar,
        openAccounts = openAccounts,
        openTransfers = { showTransfers = true },
        download = download,
        share = { sharing = it },
    )

    val backStack by nav.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination?.route

    CompositionLocalProvider(LocalAppChrome provides chrome) {
        NavigationSuiteScaffold(
            navigationSuiteItems = {
                TopLevel.entries.forEach { dest ->
                    val selected = currentRoute == dest.route || (dest == TopLevel.Files && currentRoute == Routes.FOLDER)
                    item(
                        selected = selected,
                        onClick = { nav.navigateTopLevel(dest.route) },
                        icon = { Icon(if (selected) dest.selectedIcon else dest.icon, contentDescription = null) },
                        label = { Text(stringResource(dest.label)) },
                    )
                }
            },
        ) {
            Box(Modifier.fillMaxSize()) {
                NavHost(nav, startDestination = Routes.FILES, modifier = Modifier.fillMaxSize()) {
                    composable(Routes.FILES) { DrivesScreen(session, onOpenFolder = { nav.navigate(Routes.folder(it)) }) }
                    composable(Routes.SHARED) { SharedWithMeScreen(session, onOpenFolder = { nav.navigate(Routes.folder(it)) }) }
                    composable(Routes.SETTINGS) { SettingsScreen(session, container.device) }
                    composable(Routes.FOLDER) { entry ->
                        FolderScreen(
                            session = session,
                            folderId = android.net.Uri.decode(entry.arguments?.getString("id").orEmpty()),
                            onOpenFolder = { nav.navigate(Routes.folder(it)) },
                            onBack = { nav.popBackStack() },
                        )
                    }
                }
                SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
            }
        }

        sharing?.let { ShareSheet(session, it, onDismiss = { sharing = null }) }
        if (showTransfers) {
            TransfersSheet(
                transfers = transfers,
                onCancel = { container.downloads.cancel(it.id) },
                onDismissTransfer = { container.downloads.dismiss(it.id) },
                onClearFinished = container.downloads::clearFinished,
                onClose = { showTransfers = false },
            )
        }
    }
}

private fun NavHostController.navigateTopLevel(route: String) {
    navigate(route) {
        popUpTo(graph.findStartDestination().id) { saveState = true }
        launchSingleTop = true
        restoreState = true
    }
}
