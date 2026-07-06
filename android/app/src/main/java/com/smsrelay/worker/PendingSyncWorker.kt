package com.smsrelay.worker

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters

class PendingSyncWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val retryIds = PendingMessageSync.syncAllPending(applicationContext)
        return if (retryIds.isEmpty()) Result.success() else Result.retry()
    }
}
