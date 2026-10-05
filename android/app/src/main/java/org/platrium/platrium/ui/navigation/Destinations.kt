package org.platrium.platrium.ui.navigation

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material.icons.filled.FolderShared
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.FolderShared
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.ui.graphics.vector.ImageVector
import org.platrium.platrium.R

object Routes {
    const val FILES = "files"
    const val SHARED = "shared"
    const val SETTINGS = "settings"
    const val FOLDER = "folder/{id}"
    fun folder(id: String) = "folder/${android.net.Uri.encode(id)}"
}

/** The top-level destinations shown in the navigation bar (or rail on wide screens). */
enum class TopLevel(val route: String, val label: Int, val icon: ImageVector, val selectedIcon: ImageVector) {
    Files(Routes.FILES, R.string.nav_files, Icons.Outlined.Folder, Icons.Filled.Folder),
    Shared(Routes.SHARED, R.string.nav_shared, Icons.Outlined.FolderShared, Icons.Filled.FolderShared),
    Settings(Routes.SETTINGS, R.string.nav_settings, Icons.Outlined.Settings, Icons.Filled.Settings),
}
