package org.platrium.platrium.core.account

import java.net.URI

sealed class ServerUrlException(message: String) : Exception(message) {
    object Empty : ServerUrlException("Enter your server's address.")
    object Invalid : ServerUrlException("That doesn't look like a valid server address.")
    class UnsupportedScheme(scheme: String) :
        ServerUrlException("Only http and https servers are supported, not “$scheme”.")
}

object ServerUrl {
    /**
     * Normalizes what the user typed: trims, adds a scheme (`http://` for local
     * and LAN hosts, `https://` otherwise), lower-cases scheme and host, and
     * drops the query, fragment and trailing slashes.
     */
    fun normalize(input: String): String {
        val trimmed = input.trim()
        if (trimmed.isEmpty()) throw ServerUrlException.Empty

        val withScheme = if (trimmed.contains("://")) {
            trimmed
        } else {
            val host = trimmed.substringBefore('/')
            (if (isLocalHost(host)) "http://" else "https://") + trimmed
        }

        val uri = try {
            URI(withScheme)
        } catch (_: Exception) {
            throw ServerUrlException.Invalid
        }
        val scheme = uri.scheme?.lowercase() ?: throw ServerUrlException.Invalid
        val host = uri.host?.lowercase()?.takeIf { it.isNotEmpty() } ?: throw ServerUrlException.Invalid
        if (scheme != "http" && scheme != "https") throw ServerUrlException.UnsupportedScheme(scheme)
        if (uri.userInfo != null) throw ServerUrlException.Invalid

        val hostPart = if (host.contains(':')) "[$host]" else host
        val port = if (uri.port >= 0) ":${uri.port}" else ""
        return "$scheme://$hostPart$port${(uri.rawPath ?: "").trimEnd('/')}"
    }

    internal fun isLocalHost(hostAndPort: String): Boolean {
        val host = hostAndPort.substringBefore(':').lowercase()
        if (host == "localhost" || host.endsWith(".local") || !host.contains('.')) return true
        val parts = host.split('.')
        return parts.size == 4 && parts.all { p -> p.toIntOrNull()?.let { it in 0..255 } == true }
    }
}
