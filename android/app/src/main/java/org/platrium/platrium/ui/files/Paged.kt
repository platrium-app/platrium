package org.platrium.platrium.ui.files

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.platrium.platrium.data.files.Page

data class PagedState<T>(
    val items: List<T> = emptyList(),
    /** First load, with nothing to show yet. */
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val loadingMore: Boolean = false,
    val error: String? = null,
    val hasNext: Boolean = false,
    val cursor: String? = null,
)

/** Cursor paging for a list screen: first page, refresh, and more on demand. */
class PagedLoader<T>(
    private val scope: CoroutineScope,
    private val fetch: suspend (cursor: String?) -> Page<T>,
) {
    private val _state = MutableStateFlow(PagedState<T>())
    val state: StateFlow<PagedState<T>> = _state.asStateFlow()

    init {
        load(reset = true)
    }

    fun refresh() {
        _state.update { it.copy(refreshing = true) }
        load(reset = true)
    }

    fun loadMore() {
        val s = _state.value
        if (s.loading || s.refreshing || s.loadingMore || !s.hasNext) return
        _state.update { it.copy(loadingMore = true) }
        load(reset = false)
    }

    private fun load(reset: Boolean) {
        val cursor = if (reset) null else _state.value.cursor
        scope.launch {
            try {
                val page = fetch(cursor)
                _state.update {
                    PagedState(
                        items = if (reset) page.items else it.items + page.items,
                        loading = false,
                        hasNext = page.hasNextPage,
                        cursor = page.endCursor,
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(loading = false, refreshing = false, loadingMore = false, error = e.message ?: "Something went wrong.") }
            }
        }
    }
}
