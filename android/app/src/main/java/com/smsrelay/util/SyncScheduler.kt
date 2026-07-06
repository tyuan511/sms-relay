package com.smsrelay.util

import android.content.Context
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import com.smsrelay.worker.HeartbeatWorker
import com.smsrelay.worker.PendingSyncWorker
import com.smsrelay.worker.UploadSmsWorker
import java.util.concurrent.TimeUnit

object SyncScheduler {
    private const val PERIODIC_SYNC = "pending_sync_periodic"
    private const val IMMEDIATE_SYNC = "pending_sync_immediate"
    private const val HEARTBEAT = "device_heartbeat"
    private const val IMMEDIATE_HEARTBEAT = "device_heartbeat_immediate"

    fun schedulePeriodicSync(context: Context) {
        val request = PeriodicWorkRequestBuilder<PendingSyncWorker>(15, TimeUnit.MINUTES)
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build(),
            )
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
            .build()

        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            PERIODIC_SYNC,
            ExistingPeriodicWorkPolicy.KEEP,
            request,
        )
    }

    fun scheduleHeartbeat(context: Context) {
        val request = PeriodicWorkRequestBuilder<HeartbeatWorker>(15, TimeUnit.MINUTES)
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build(),
            )
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
            .build()

        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            HEARTBEAT,
            ExistingPeriodicWorkPolicy.KEEP,
            request,
        )
    }

    /** Safe to call from BOOT_COMPLETED — does not start a foreground service. */
    fun scheduleOnBoot(context: Context) {
        schedulePeriodicSync(context)
        scheduleHeartbeat(context)
        enqueueImmediateSync(context)
        enqueueImmediateHeartbeat(context)
    }

    fun enqueueImmediateSync(context: Context) {
        val request = OneTimeWorkRequestBuilder<PendingSyncWorker>()
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build(),
            )
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
            .build()

        WorkManager.getInstance(context).enqueueUniqueWork(
            IMMEDIATE_SYNC,
            ExistingWorkPolicy.REPLACE,
            request,
        )
    }

    fun enqueueImmediateHeartbeat(context: Context) {
        val request = OneTimeWorkRequestBuilder<HeartbeatWorker>()
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build(),
            )
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
            .build()

        WorkManager.getInstance(context).enqueueUniqueWork(
            IMMEDIATE_HEARTBEAT,
            ExistingWorkPolicy.REPLACE,
            request,
        )
    }

    /** WorkManager fallback when foreground service upload fails or cannot start. */
    fun enqueueUploadRetry(context: Context, messageId: Long) {
        val request = OneTimeWorkRequestBuilder<UploadSmsWorker>()
            .setInputData(
                androidx.work.workDataOf(UploadSmsWorker.KEY_MESSAGE_ID to messageId),
            )
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build(),
            )
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
            .build()

        WorkManager.getInstance(context).enqueue(request)
    }
}
