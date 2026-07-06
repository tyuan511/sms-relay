package com.smsrelay.api

import android.content.Context
import com.smsrelay.data.Prefs
import retrofit2.HttpException
import java.io.IOException

object DeviceHeartbeat {
    suspend fun send(context: Context): Result {
        val prefs = Prefs(context)
        if (!prefs.isConfigured()) return Result.Skipped

        return try {
            doSend(prefs)
        } catch (e: HttpException) {
            when (e.code()) {
                401, 403 -> {
                    try {
                        ApiClient.refreshDeviceToken(prefs)
                        doSend(prefs)
                    } catch (ex: Exception) {
                        Result.Failure(ex.message ?: "auth refresh failed")
                    }
                }
                else -> Result.Failure("HTTP ${e.code()}")
            }
        } catch (e: IOException) {
            Result.Failure(e.message ?: "network error")
        } catch (e: Exception) {
            Result.Failure(e.message ?: "unknown error")
        }
    }

    private suspend fun doSend(prefs: Prefs): Result {
        val token = ApiClient.ensureDeviceToken(prefs)
        val api = ApiClient.create(prefs.serverUrl)
        val response = api.heartbeat("Bearer $token")
        return if (response.isSuccessful) {
            Result.Success
        } else {
            throw HttpException(response)
        }
    }

    sealed class Result {
        data object Success : Result()
        data object Skipped : Result()
        data class Failure(val reason: String) : Result()
    }
}
