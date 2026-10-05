package org.platrium.platrium.core.transfer

import android.net.Uri

/** What to do with a file once it has been downloaded. */
enum class DownloadAction {
    /** Save to the device's Downloads folder. */
    SAVE,

    /** Download to the app's cache, then open it in another app. */
    OPEN,

    /** Download to the app's cache, then hand it to the share sheet. */
    SHARE,
}

sealed interface TransferState {
    data object Preparing : TransferState
    data object Transferring : TransferState
    data object Completed : TransferState
    data class Failed(val message: String) : TransferState
    data object Cancelled : TransferState

    val isActive: Boolean get() = this is Preparing || this is Transferring
}

data class Transfer(
    val id: String,
    val fileId: String,
    val name: String,
    val action: DownloadAction,
    val state: TransferState = TransferState.Preparing,
    val bytesTransferred: Long = 0,
    val totalBytes: Long = 0,
    /** Where the finished file is; set when [state] is [TransferState.Completed]. */
    val uri: Uri? = null,
    val mimeType: String? = null,
) {
    /** 0f..1f, or null while the size isn't known. */
    val progress: Float? get() = if (totalBytes > 0) (bytesTransferred.toFloat() / totalBytes).coerceIn(0f, 1f) else null
}
