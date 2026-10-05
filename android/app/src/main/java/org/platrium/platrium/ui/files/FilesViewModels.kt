package org.platrium.platrium.ui.files

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.platrium.platrium.data.files.Drive
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.data.files.FilesRepository
import org.platrium.platrium.data.files.ItemDetails
import org.platrium.platrium.data.files.SharedItem

data class DrivesState(
    val drives: List<Drive> = emptyList(),
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val error: String? = null,
)

class DrivesViewModel(private val files: FilesRepository) : ViewModel() {
    private val _state = MutableStateFlow(DrivesState())
    val state: StateFlow<DrivesState> = _state.asStateFlow()

    init {
        load()
    }

    fun refresh() {
        _state.update { it.copy(refreshing = true) }
        load()
    }

    private fun load() {
        viewModelScope.launch {
            try {
                _state.value = DrivesState(drives = files.drives(), loading = false)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(loading = false, refreshing = false, error = e.message ?: "Something went wrong.") }
            }
        }
    }
}

class SharedWithMeViewModel(private val files: FilesRepository) : ViewModel() {
    private val paged = PagedLoader<SharedItem>(viewModelScope) { files.sharedWithMe(it) }
    val state: StateFlow<PagedState<SharedItem>> = paged.state
    fun refresh() = paged.refresh()
    fun loadMore() = paged.loadMore()
}

class FolderViewModel(private val files: FilesRepository, private val folderId: String) : ViewModel() {
    private val paged = PagedLoader<DriveItem>(viewModelScope) { files.folderContents(folderId, it) }
    val contents: StateFlow<PagedState<DriveItem>> = paged.state

    private val _details = MutableStateFlow<ItemDetails?>(null)
    val details: StateFlow<ItemDetails?> = _details.asStateFlow()

    /** A one-off message for a snackbar; the screen clears it with [messageShown]. */
    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message.asStateFlow()

    init {
        viewModelScope.launch {
            try {
                _details.value = files.item(folderId)
            } catch (e: CancellationException) {
                throw e
            } catch (_: Exception) {
                // The title falls back to a generic one; the contents list shows its own error.
            }
        }
    }

    fun refresh() = paged.refresh()
    fun loadMore() = paged.loadMore()
    fun messageShown() { _message.value = null }

    fun createFolder(name: String) = change { files.createFolder(folderId, name) }
    fun rename(item: DriveItem, name: String) = change { files.rename(item.id, name) }
    fun delete(item: DriveItem) = change { files.delete(item.id) }

    private fun change(block: suspend () -> Unit) {
        viewModelScope.launch {
            try {
                block()
                paged.refresh()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _message.value = e.message ?: "Something went wrong."
            }
        }
    }
}
