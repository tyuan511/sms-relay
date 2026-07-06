package com.smsrelay.api

import com.smsrelay.data.Prefs
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.TimeUnit

object ApiClient {
    private val logging = HttpLoggingInterceptor().apply {
        level = HttpLoggingInterceptor.Level.BASIC
    }

    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .addInterceptor(logging)
        .build()

    fun create(baseUrl: String): SmsRelayApi {
        val url = if (baseUrl.endsWith("/")) baseUrl else "$baseUrl/"
        return Retrofit.Builder()
            .baseUrl(url)
            .client(httpClient)
            .addConverterFactory(GsonConverterFactory.create())
            .build()
            .create(SmsRelayApi::class.java)
    }

    suspend fun ensureDeviceToken(prefs: Prefs): String {
        val existing = prefs.deviceToken
        if (existing.isNotBlank()) return existing

        val api = create(prefs.serverUrl)
        val deviceName = android.os.Build.MODEL ?: "Android"
        val response = api.authDevice(
            DeviceAuthRequest(
                masterPassword = prefs.masterPassword,
                deviceName = deviceName,
                deviceClientId = prefs.deviceClientId,
            ),
        )
        prefs.deviceToken = response.deviceToken
        prefs.deviceId = response.deviceId
        return response.deviceToken
    }

    suspend fun refreshDeviceToken(prefs: Prefs): String {
        prefs.deviceToken = ""
        return ensureDeviceToken(prefs)
    }
}
