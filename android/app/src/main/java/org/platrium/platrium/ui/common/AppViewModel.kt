package org.platrium.platrium.ui.common

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import org.platrium.platrium.AppContainer
import org.platrium.platrium.PlatriumApplication

@Composable
fun rememberContainer(): AppContainer = (LocalContext.current.applicationContext as PlatriumApplication).container

/** A ViewModel built from the app container; [key] separates instances (e.g. per account and folder). */
@Composable
inline fun <reified VM : ViewModel> appViewModel(key: String? = null, noinline create: (AppContainer) -> VM): VM {
    val container = rememberContainer()
    return viewModel(key = key, factory = viewModelFactory { initializer { create(container) } })
}
