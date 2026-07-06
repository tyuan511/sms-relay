package com.smsrelay.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import com.smsrelay.R
import com.smsrelay.api.DeviceHeartbeat
import com.smsrelay.data.Prefs
import com.smsrelay.ui.MainActivity
import com.smsrelay.util.SyncScheduler
import com.smsrelay.worker.PendingMessageSync
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.util.concurrent.atomic.AtomicInteger

class SmsMonitorService : Service() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val activeJobs = AtomicInteger(0)
    private var heartbeatJob: Job? = null
    @Volatile
    private var monitoring = false
    @Volatile
    private var timeoutMessageId: Long = -1L

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val messageId = intent?.getLongExtra(EXTRA_MESSAGE_ID, -1L) ?: -1L
        if (messageId < 0) {
            if (usesAndroid15ForegroundServiceLimits()) {
                SyncScheduler.scheduleHeartbeat(this)
                SyncScheduler.enqueueImmediateHeartbeat(this)
                stopSelfResult(startId)
                return START_NOT_STICKY
            }

            createChannel()
            startForeground(NOTIFICATION_ID, buildNotification(R.string.service_running))
            monitoring = true
            if (heartbeatJob?.isActive != true) {
                startHeartbeat()
            }
            return START_STICKY
        }

        createChannel()
        startForeground(NOTIFICATION_ID, buildNotification(R.string.service_uploading))
        activeJobs.incrementAndGet()
        timeoutMessageId = messageId

        scope.launch {
            try {
                when (val outcome = PendingMessageSync.syncMessage(this@SmsMonitorService, messageId)) {
                    is PendingMessageSync.Outcome.RetryableFailure -> {
                        Log.w(TAG, "upload retry scheduled for $messageId: ${outcome.reason}")
                        SyncScheduler.enqueueUploadRetry(this@SmsMonitorService, messageId)
                    }
                    is PendingMessageSync.Outcome.PermanentFailure -> {
                        Log.w(TAG, "upload failed for $messageId: ${outcome.reason}")
                    }
                    else -> Unit
                }
            } finally {
                finishUpload(startId)
            }
        }

        return START_NOT_STICKY
    }

    override fun onTimeout(startId: Int, fgsType: Int) {
        Log.w(TAG, "FGS timeout for message $timeoutMessageId")
        heartbeatJob?.cancel()
        if (timeoutMessageId >= 0) {
            SyncScheduler.enqueueUploadRetry(this, timeoutMessageId)
        }
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelfResult(startId)
    }

    override fun onDestroy() {
        heartbeatJob?.cancel()
        scope.cancel()
        super.onDestroy()
    }

    private fun finishUpload(startId: Int) {
        if (activeJobs.decrementAndGet() == 0) {
            if (monitoring && !usesAndroid15ForegroundServiceLimits()) {
                startForeground(NOTIFICATION_ID, buildNotification(R.string.service_running))
                return
            }
            ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        }
        stopSelfResult(startId)
    }

    private fun startHeartbeat() {
        heartbeatJob?.cancel()
        heartbeatJob = scope.launch {
            while (isActive) {
                if (Prefs(this@SmsMonitorService).isConfigured()) {
                    when (val result = DeviceHeartbeat.send(this@SmsMonitorService)) {
                        is DeviceHeartbeat.Result.Failure -> {
                            Log.w(TAG, "heartbeat failed: ${result.reason}")
                        }
                        else -> Unit
                    }
                }
                delay(HEARTBEAT_INTERVAL_MS)
            }
        }
    }

    private fun createChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.service_channel),
                NotificationManager.IMPORTANCE_LOW,
            )
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
    }

    private fun buildNotification(textRes: Int): Notification {
        val intent = Intent(this, MainActivity::class.java)
        val pending = PendingIntent.getActivity(
            this,
            0,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.app_name))
            .setContentText(getString(textRes))
            .setSmallIcon(R.drawable.ic_launcher)
            .setContentIntent(pending)
            .setOngoing(true)
            .build()
    }

    companion object {
        private const val TAG = "SmsMonitorService"
        private const val CHANNEL_ID = "sms_monitor"
        private const val NOTIFICATION_ID = 1
        private const val HEARTBEAT_INTERVAL_MS = 5 * 60 * 1000L
        const val EXTRA_MESSAGE_ID = "message_id"

        fun startMonitoring(context: Context) {
            if (usesAndroid15ForegroundServiceLimits()) {
                SyncScheduler.scheduleHeartbeat(context)
                SyncScheduler.enqueueImmediateHeartbeat(context)
                return
            }
            startService(context, Intent(context, SmsMonitorService::class.java))
        }

        fun requestUpload(context: Context, messageId: Long) {
            val intent = Intent(context, SmsMonitorService::class.java).apply {
                putExtra(EXTRA_MESSAGE_ID, messageId)
            }
            startService(context, intent)
        }

        private fun startService(context: Context, intent: Intent) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        private fun usesAndroid15ForegroundServiceLimits(): Boolean {
            return Build.VERSION.SDK_INT >= Build.VERSION_CODES.VANILLA_ICE_CREAM
        }
    }
}
