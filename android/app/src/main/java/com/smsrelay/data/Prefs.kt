package com.smsrelay.data

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.util.UUID

class Prefs(context: Context) {
    private val prefs = EncryptedSharedPreferences.create(
        context,
        "sms_relay_secure",
        MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    var serverUrl: String
        get() = prefs.getString(KEY_SERVER_URL, "") ?: ""
        set(value) = prefs.edit().putString(KEY_SERVER_URL, com.smsrelay.util.ServerUrl.normalize(value)).apply()

    var masterPassword: String
        get() = prefs.getString(KEY_MASTER_PASSWORD, "") ?: ""
        set(value) = prefs.edit().putString(KEY_MASTER_PASSWORD, value).apply()

    var deviceToken: String
        get() = prefs.getString(KEY_DEVICE_TOKEN, "") ?: ""
        set(value) = prefs.edit().putString(KEY_DEVICE_TOKEN, value).apply()

    var deviceId: String
        get() = prefs.getString(KEY_DEVICE_ID, "") ?: ""
        set(value) = prefs.edit().putString(KEY_DEVICE_ID, value).apply()

    val deviceClientId: String
        get() {
            val existing = prefs.getString(KEY_DEVICE_CLIENT_ID, "") ?: ""
            if (existing.isNotBlank()) return existing
            val generated = UUID.randomUUID().toString()
            prefs.edit().putString(KEY_DEVICE_CLIENT_ID, generated).apply()
            return generated
        }

    var lastUploadAt: Long
        get() = prefs.getLong(KEY_LAST_UPLOAD, 0L)
        set(value) = prefs.edit().putLong(KEY_LAST_UPLOAD, value).apply()

    fun isConfigured(): Boolean = serverUrl.isNotBlank() && masterPassword.isNotBlank()

    fun clearDeviceAuth() {
        prefs.edit()
            .remove(KEY_DEVICE_TOKEN)
            .remove(KEY_DEVICE_ID)
            .apply()
    }

    companion object {
        private const val KEY_SERVER_URL = "server_url"
        private const val KEY_MASTER_PASSWORD = "master_password"
        private const val KEY_DEVICE_TOKEN = "device_token"
        private const val KEY_DEVICE_ID = "device_id"
        private const val KEY_DEVICE_CLIENT_ID = "device_client_id"
        private const val KEY_LAST_UPLOAD = "last_upload_at"
    }
}
