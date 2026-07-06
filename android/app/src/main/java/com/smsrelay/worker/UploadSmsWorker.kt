package com.smsrelay.worker

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters

class UploadSmsWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val messageId = inputData.getLong(KEY_MESSAGE_ID, -1L)
        if (messageId < 0) return Result.failure()

        return when (val outcome = PendingMessageSync.syncMessage(applicationContext, messageId)) {
            PendingMessageSync.Outcome.NotFound,
            PendingMessageSync.Outcome.AlreadyDone,
            PendingMessageSync.Outcome.Success,
            -> Result.success()
            is PendingMessageSync.Outcome.PermanentFailure -> Result.failure()
            is PendingMessageSync.Outcome.RetryableFailure -> Result.retry()
        }
    }

    companion object {
        const val KEY_MESSAGE_ID = "message_id"
    }
}
