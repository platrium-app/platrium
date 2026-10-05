package org.platrium.platrium.core.transfer

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.core.content.ContextCompat
import androidx.lifecycle.LifecycleService
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.launch
import org.platrium.platrium.MainActivity
import org.platrium.platrium.PlatriumApplication
import org.platrium.platrium.R

/**
 * Keeps the process alive, with a progress notification, while downloads run.
 * The work itself is owned by [DownloadManager].
 */
class DownloadService : LifecycleService() {
    override fun onCreate() {
        super.onCreate()
        val manager = (application as PlatriumApplication).container.downloads

        getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL_ID, getString(R.string.downloads_channel), NotificationManager.IMPORTANCE_LOW),
        )
        startForeground(NOTIFICATION_ID, notification(emptyList()), ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)

        lifecycleScope.launch {
            manager.transfers.collect { transfers ->
                val active = transfers.filter { it.state.isActive }
                if (active.isEmpty()) {
                    stopForeground(STOP_FOREGROUND_REMOVE)
                    stopSelf()
                } else {
                    getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, notification(active))
                }
            }
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        super.onStartCommand(intent, flags, startId)
        return START_NOT_STICKY
    }

    override fun onBind(intent: Intent): IBinder? = super.onBind(intent)

    private fun notification(active: List<Transfer>): Notification {
        val title = when (active.size) {
            0 -> getString(R.string.downloads_notification_title)
            1 -> active.first().name
            else -> resources.getQuantityString(R.plurals.downloads_notification_count, active.size, active.size)
        }
        val known = active.filter { it.totalBytes > 0 }
        val total = known.sumOf { it.totalBytes }
        val done = known.sumOf { it.bytesTransferred }
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_sys_download)
            .setContentTitle(title)
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setProgress(100, if (total > 0) (done * 100 / total).toInt() else 0, total == 0L)
            .setContentIntent(
                android.app.PendingIntent.getActivity(
                    this, 0, Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
                    android.app.PendingIntent.FLAG_IMMUTABLE,
                ),
            )
            .build()
    }

    companion object {
        private const val CHANNEL_ID = "downloads"
        private const val NOTIFICATION_ID = 1

        fun start(context: Context) {
            ContextCompat.startForegroundService(context, Intent(context, DownloadService::class.java))
        }
    }
}
