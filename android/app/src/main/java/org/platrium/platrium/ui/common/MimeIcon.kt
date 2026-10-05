package org.platrium.platrium.ui.common

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Article
import androidx.compose.material.icons.outlined.AudioFile
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.FolderShared
import androidx.compose.material.icons.outlined.FolderZip
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.InsertDriveFile
import androidx.compose.material.icons.outlined.PictureAsPdf
import androidx.compose.material.icons.outlined.TableChart
import androidx.compose.material.icons.outlined.VideoFile
import androidx.compose.ui.graphics.vector.ImageVector

fun iconFor(isFolder: Boolean, mimeType: String?): ImageVector {
    if (isFolder) return Icons.Outlined.Folder
    val mime = mimeType.orEmpty().lowercase()
    return when {
        mime.startsWith("image/") -> Icons.Outlined.Image
        mime.startsWith("video/") -> Icons.Outlined.VideoFile
        mime.startsWith("audio/") -> Icons.Outlined.AudioFile
        mime == "application/pdf" -> Icons.Outlined.PictureAsPdf
        mime.contains("zip") || mime.contains("compressed") || mime.contains("tar") -> Icons.Outlined.FolderZip
        mime.contains("spreadsheet") || mime.contains("excel") || mime == "text/csv" -> Icons.Outlined.TableChart
        mime.startsWith("text/") -> Icons.Outlined.Article
        mime.contains("word") || mime.contains("document") || mime.contains("presentation") -> Icons.Outlined.Description
        else -> Icons.Outlined.InsertDriveFile
    }
}

val SharedFolderIcon: ImageVector get() = Icons.Outlined.FolderShared
