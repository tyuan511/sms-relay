package com.smsrelay.worker

import android.content.Context
import android.util.Log
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.smsrelay.api.DeviceHeartbeat
import com.smsrelay.data.Prefs

class HeartbeatWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        if (!Prefs(applicationContext).isConfigured()) return Result.success()

        return when (val result = DeviceHeartbeat.send(applicationContext)) {
            DeviceHeartbeat.Result.Success,
            DeviceHeartbeat.Result.Skipped,
            -> Result.success()
            is DeviceHeartbeat.Result.Failure -> {
                Log.w(TAG, "heartbeat failed: ${result.reason}")
                Result.retry()
            }
        }
    }

    companion object {
        private const val TAG = "HeartbeatWorker"
    }
}
