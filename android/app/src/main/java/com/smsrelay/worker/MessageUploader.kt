package com.smsrelay.worker

import android.content.Context
import com.smsrelay.api.ApiClient
import com.smsrelay.api.InboundRequest
import com.smsrelay.data.Prefs
import retrofit2.HttpException
import java.io.IOException

object MessageUploader {
    suspend fun uploadMessage(
        context: Context,
        message: com.smsrelay.data.PendingMessage,
    ): UploadResult {
        val prefs = Prefs(context)
        if (!prefs.isConfigured()) return UploadResult.PermanentFailure("not configured")

        return try {
            doUpload(prefs, message)
            prefs.lastUploadAt = System.currentTimeMillis()
            UploadResult.Success
        } catch (e: HttpException) {
            when (e.code()) {
                401, 403 -> {
                    try {
                        ApiClient.refreshDeviceToken(prefs)
                        doUpload(prefs, message)
                        prefs.lastUploadAt = System.currentTimeMillis()
                        UploadResult.Success
                    } catch (ex: Exception) {
                        UploadResult.RetryableFailure(ex.message ?: "auth refresh failed")
                    }
                }
                in 400..499 -> UploadResult.PermanentFailure("HTTP ${e.code()}")
                else -> UploadResult.RetryableFailure("HTTP ${e.code()}")
            }
        } catch (e: IOException) {
            UploadResult.RetryableFailure(e.message ?: "network error")
        } catch (e: Exception) {
            UploadResult.RetryableFailure(e.message ?: "unknown error")
        }
    }

    private suspend fun doUpload(prefs: Prefs, message: com.smsrelay.data.PendingMessage) {
        val token = ApiClient.ensureDeviceToken(prefs)
        val api = ApiClient.create(prefs.serverUrl)
        val deviceName = android.os.Build.MODEL ?: "Android"
        val response = api.uploadMessage(
            "Bearer $token",
            InboundRequest(
                sender = message.sender,
                body = message.body,
                receivedAt = message.receivedAt,
                deviceName = deviceName,
                clientMessageId = message.clientMessageId,
            ),
        )
        if (!response.isSuccessful) {
            throw HttpException(response)
        }
    }
}

sealed class UploadResult {
    data object Success : UploadResult()
    data class RetryableFailure(val reason: String) : UploadResult()
    data class PermanentFailure(val reason: String) : UploadResult()
}
