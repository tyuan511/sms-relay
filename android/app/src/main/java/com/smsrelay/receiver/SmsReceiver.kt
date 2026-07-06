package com.smsrelay.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import android.util.Log
import com.smsrelay.data.AppDatabase
import com.smsrelay.data.PendingMessage
import com.smsrelay.service.SmsMonitorService
import com.smsrelay.util.SyncScheduler
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import java.time.Instant

class SmsReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return

        val messages = Telephony.Sms.Intents.getMessagesFromIntent(intent)
        if (messages.isNullOrEmpty()) return

        val body = messages.joinToString("") { it.messageBody ?: "" }
        val sender = messages.firstOrNull()?.originatingAddress ?: "unknown"
        val receivedAt = Instant.now().toString()
        val appContext = context.applicationContext

        val pendingResult = goAsync()
        CoroutineScope(SupervisorJob() + Dispatchers.IO).launch {
            try {
                val messageId = AppDatabase.get(appContext).pendingMessageDao().insert(
                    PendingMessage(
                        sender = sender,
                        body = body,
                        receivedAt = receivedAt,
                    ),
                )

                try {
                    SmsMonitorService.requestUpload(appContext, messageId)
                } catch (e: Exception) {
                    Log.w(TAG, "foreground service unavailable, falling back to WorkManager", e)
                    SyncScheduler.enqueueUploadRetry(appContext, messageId)
                }
            } catch (e: Exception) {
                Log.e(TAG, "failed to queue SMS", e)
                SyncScheduler.enqueueImmediateSync(appContext)
            } finally {
                pendingResult.finish()
            }
        }
    }

    companion object {
        private const val TAG = "SmsReceiver"
    }
}
