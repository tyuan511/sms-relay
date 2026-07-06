package com.smsrelay.worker

import android.content.Context
import com.smsrelay.data.AppDatabase
import com.smsrelay.data.PendingMessage
import com.smsrelay.data.PendingMessageDao

object PendingMessageSync {
    sealed class Outcome {
        data object NotFound : Outcome()
        data object AlreadyDone : Outcome()
        data object Success : Outcome()
        data class PermanentFailure(val reason: String) : Outcome()
        data class RetryableFailure(val reason: String) : Outcome()
    }

    suspend fun syncMessage(context: Context, messageId: Long): Outcome {
        val dao = AppDatabase.get(context).pendingMessageDao()
        val message = dao.getById(messageId) ?: return Outcome.NotFound
        if (message.status == PendingMessage.STATUS_DONE) return Outcome.AlreadyDone
        return applyUpload(context, dao, message)
    }

    suspend fun syncAllPending(context: Context): List<Long> {
        val dao = AppDatabase.get(context).pendingMessageDao()
        val pending = dao.listByStatus(PendingMessage.STATUS_PENDING)
        val retryIds = mutableListOf<Long>()
        for (message in pending) {
            when (val outcome = applyUpload(context, dao, message)) {
                is Outcome.RetryableFailure -> retryIds.add(message.id)
                else -> Unit
            }
        }
        return retryIds
    }

    private suspend fun applyUpload(
        context: Context,
        dao: PendingMessageDao,
        message: PendingMessage,
    ): Outcome {
        return when (val result = MessageUploader.uploadMessage(context, message)) {
            is UploadResult.Success -> {
                dao.update(message.copy(status = PendingMessage.STATUS_DONE, lastError = null))
                Outcome.Success
            }
            is UploadResult.PermanentFailure -> {
                dao.update(
                    message.copy(
                        status = PendingMessage.STATUS_FAILED,
                        lastError = result.reason,
                    ),
                )
                Outcome.PermanentFailure(result.reason)
            }
            is UploadResult.RetryableFailure -> {
                dao.update(
                    message.copy(
                        status = PendingMessage.STATUS_PENDING,
                        retryCount = message.retryCount + 1,
                        lastError = result.reason,
                    ),
                )
                Outcome.RetryableFailure(result.reason)
            }
        }
    }
}
