package org.platrium.platrium.core.transfer

import android.content.ContentValues
import android.content.Context
import android.net.Uri
import android.os.Environment
import android.os.ParcelFileDescriptor
import android.provider.MediaStore
import androidx.core.content.FileProvider
import java.io.File
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.platrium.platrium.core.network.AccountSession
import uniffi.platrium_sdk.DownloadDestination
import uniffi.platrium_sdk.NetTransferEvent
import uniffi.platrium_sdk.PlatriumClient
import uniffi.platrium_sdk.TransferDirection
import uniffi.platrium_sdk.TransferEventListener
import uniffi.platrium_sdk.TransferMetadata
import uniffi.platrium_sdk.TransferStatus

/**
 * Downloads files through the SDK, which streams them in chunks straight into
 * a file descriptor we hand it. Lives for the process; [DownloadService] keeps
 * the process alive while something is running.
 */
class DownloadManager(private val context: Context, private val scope: CoroutineScope) {
    private val _transfers = MutableStateFlow<List<Transfer>>(emptyList())
    val transfers: StateFlow<List<Transfer>> = _transfers.asStateFlow()

    private val _completed = MutableSharedFlow<Transfer>(extraBufferCapacity = 8)

    /** Emits each transfer as it finishes successfully. */
    val completed: SharedFlow<Transfer> = _completed.asSharedFlow()

    private val jobs = ConcurrentHashMap<String, Job>()
    private val sdkTransferIds = ConcurrentHashMap<String, String>() // SDK transfer id -> our id

    /** Subscribes to progress; called once for each new SDK client. */
    fun attach(client: PlatriumClient) {
        client.files().onTransferEvent(object : TransferEventListener {
            override fun onEvent(event: NetTransferEvent) = this@DownloadManager.onEvent(event)
        })
    }

    fun download(session: AccountSession, fileId: String, name: String, action: DownloadAction) {
        val id = UUID.randomUUID().toString()
        _transfers.update { listOf(Transfer(id, fileId, name, action)) + it }
        DownloadService.start(context)

        jobs[id] = scope.launch(Dispatchers.IO) {
            var target: Target? = null
            try {
                val download = session.sdk.files().createDownloadSession(fileId)
                sdkTransferIds[download.sessionId()] = id
                update(id) { it.copy(name = download.fileName(), totalBytes = download.fileSize().toLong(), mimeType = download.mimeType()) }

                target = open(id, download.fileName(), download.mimeType(), action)
                // The SDK takes ownership of the descriptor and closes it when done.
                val destination = DownloadDestination(target.descriptor.detachFd())
                download.streamTo(destination)

                val uri = target.finish()
                val done = update(id) {
                    it.copy(state = TransferState.Completed, bytesTransferred = it.totalBytes, uri = uri)
                }
                if (done != null) _completed.tryEmit(done)
            } catch (e: CancellationException) {
                target?.discard()
                update(id) { it.copy(state = TransferState.Cancelled) }
                throw e
            } catch (e: Exception) {
                target?.discard()
                update(id) { it.copy(state = TransferState.Failed(e.message ?: "The download failed.")) }
            } finally {
                jobs.remove(id)
            }
        }
    }

    fun cancel(id: String) {
        jobs[id]?.cancel()
    }

    /** Removes a finished, failed or cancelled transfer from the list. */
    fun dismiss(id: String) {
        _transfers.update { list -> list.filterNot { it.id == id && !it.state.isActive } }
    }

    fun clearFinished() {
        _transfers.update { list -> list.filter { it.state.isActive } }
    }

    private fun onEvent(event: NetTransferEvent) {
        if (event.direction != TransferDirection.DOWNLOAD) return
        val id = sdkTransferIds[event.transferId]
            ?: (event.metadata as? TransferMetadata.FileDownloadEvent)?.let { meta ->
                _transfers.value.firstOrNull { it.fileId == meta.fileId && it.state.isActive }?.id
            }
            ?: return
        update(id) { t ->
            if (!t.state.isActive) return@update t
            when (val status = event.status) {
                is TransferStatus.Preparing -> t
                is TransferStatus.Transferring -> t.copy(
                    state = TransferState.Transferring,
                    bytesTransferred = event.bytesTransferred.toLong(),
                    totalBytes = event.totalBytes.toLong().takeIf { it > 0 } ?: t.totalBytes,
                )
                // Completion and errors are handled where the download is awaited.
                is TransferStatus.Completed, is TransferStatus.Error, is TransferStatus.Cancelled -> t
            }
        }
    }

    private fun update(id: String, change: (Transfer) -> Transfer): Transfer? {
        var result: Transfer? = null
        _transfers.update { list ->
            list.map { if (it.id == id) change(it).also { updated -> result = updated } else it }
        }
        return result
    }

    /** Somewhere to write a download, and how to finish or abandon it. */
    private class Target(
        val descriptor: ParcelFileDescriptor,
        val finish: () -> Uri,
        val discard: () -> Unit,
    )

    private fun open(id: String, fileName: String, mimeType: String, action: DownloadAction): Target {
        val safeName = fileName.replace(Regex("[\\\\/:*?\"<>|\u0000]"), "_").ifBlank { "download" }
        return when (action) {
            DownloadAction.SAVE -> {
                val resolver = context.contentResolver
                val values = ContentValues().apply {
                    put(MediaStore.Downloads.DISPLAY_NAME, safeName)
                    put(MediaStore.Downloads.MIME_TYPE, mimeType)
                    put(MediaStore.Downloads.RELATIVE_PATH, "${Environment.DIRECTORY_DOWNLOADS}/Platrium")
                    put(MediaStore.Downloads.IS_PENDING, 1)
                }
                val uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values)
                    ?: error("Couldn't create the file in Downloads.")
                val descriptor = resolver.openFileDescriptor(uri, "w")
                    ?: run { resolver.delete(uri, null, null); error("Couldn't open the file in Downloads.") }
                Target(
                    descriptor,
                    finish = {
                        resolver.update(uri, ContentValues().apply { put(MediaStore.Downloads.IS_PENDING, 0) }, null, null)
                        uri
                    },
                    discard = { resolver.delete(uri, null, null) },
                )
            }
            DownloadAction.OPEN, DownloadAction.SHARE -> {
                val file = File(File(context.cacheDir, "downloads/$id").apply { mkdirs() }, safeName)
                val descriptor = ParcelFileDescriptor.open(
                    file,
                    ParcelFileDescriptor.MODE_CREATE or ParcelFileDescriptor.MODE_TRUNCATE or ParcelFileDescriptor.MODE_WRITE_ONLY,
                )
                Target(
                    descriptor,
                    finish = { FileProvider.getUriForFile(context, "${context.packageName}.files", file) },
                    discard = { file.parentFile?.deleteRecursively() },
                )
            }
        }
    }

    /** Drops cached copies from earlier runs. */
    fun clearCache() {
        File(context.cacheDir, "downloads").deleteRecursively()
    }
}
