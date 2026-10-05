package org.platrium.platrium.core.auth

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
import java.util.concurrent.atomic.AtomicReference
import kotlinx.coroutines.CompletableDeferred

/**
 * Runs the browser step in a Custom Tab. The server redirects to
 * `platrium://auth/callback`, which [MainActivity][org.platrium.platrium.MainActivity]
 * hands to [deliver]. There's at most one sign-in at a time; the user cancels
 * by cancelling the coroutine (the sign-in screen's Cancel button).
 */
class CustomTabAuthenticator(private val context: Context) {
    private val pending = AtomicReference<CompletableDeferred<String>?>(null)

    suspend fun authenticate(authorizeUrl: String): String {
        val deferred = CompletableDeferred<String>()
        pending.getAndSet(deferred)?.cancel()
        try {
            val intent = CustomTabsIntent.Builder().setShowTitle(true).build()
            intent.intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            intent.launchUrl(context, Uri.parse(authorizeUrl))
            return deferred.await()
        } finally {
            pending.compareAndSet(deferred, null)
        }
    }

    /** Called with the redirect URL; returns whether a sign-in was waiting for it. */
    fun deliver(callbackUrl: String): Boolean = pending.getAndSet(null)?.complete(callbackUrl) ?: false
}
