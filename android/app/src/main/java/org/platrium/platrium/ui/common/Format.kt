package org.platrium.platrium.ui.common

import java.text.DateFormat
import java.time.Instant
import java.time.format.DateTimeParseException
import java.util.Date
import java.util.Locale

fun formatBytes(bytes: Long): String {
    if (bytes < 1000) return "$bytes B"
    val units = listOf("kB", "MB", "GB", "TB")
    var value = bytes / 1000.0
    var unit = 0
    while (value >= 1000 && unit < units.lastIndex) { value /= 1000; unit++ }
    return String.format(Locale.getDefault(), if (value < 10) "%.1f %s" else "%.0f %s", value, units[unit])
}

/** A server timestamp (ISO 8601) as a short local date, or the original text if it can't be read. */
fun formatDate(iso: String): String = try {
    DateFormat.getDateInstance(DateFormat.MEDIUM).format(Date.from(Instant.parse(iso)))
} catch (_: DateTimeParseException) {
    iso
}

/** `yyyy-MM-dd` of an ISO instant, for date pickers. */
fun isoToEpochMillis(iso: String?): Long? = iso?.let { runCatching { Instant.parse(it).toEpochMilli() }.getOrNull() }

/** The end of the given UTC day, as the ISO instant the server stores expiries in. */
fun endOfDayIso(epochMillisUtcDay: Long): String =
    Instant.ofEpochMilli(epochMillisUtcDay).plusSeconds(86_399).toString()
