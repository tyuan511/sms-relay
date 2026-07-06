package com.smsrelay.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build
import com.smsrelay.data.Prefs
import com.smsrelay.service.SmsMonitorService
import com.smsrelay.util.SyncScheduler

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return
        if (!Prefs(context).isConfigured()) return
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.VANILLA_ICE_CREAM) {
            SmsMonitorService.startMonitoring(context)
            return
        }
        SyncScheduler.scheduleOnBoot(context)
    }
}
