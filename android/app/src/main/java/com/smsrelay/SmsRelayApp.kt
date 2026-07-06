package com.smsrelay

import android.app.Application
import androidx.work.Configuration
import com.smsrelay.data.Prefs
import com.smsrelay.util.NetworkMonitor
import com.smsrelay.util.SyncScheduler

class SmsRelayApp : Application(), Configuration.Provider {
    private var networkMonitor: NetworkMonitor? = null

    override fun onCreate() {
        super.onCreate()
        Prefs(this).ensureDeviceClientId()
        SyncScheduler.schedulePeriodicSync(this)
        if (Prefs(this).isConfigured()) {
            SyncScheduler.scheduleHeartbeat(this)
        }
        networkMonitor = NetworkMonitor(this).also { it.start() }
    }

    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder()
            .setMinimumLoggingLevel(android.util.Log.INFO)
            .build()
}
