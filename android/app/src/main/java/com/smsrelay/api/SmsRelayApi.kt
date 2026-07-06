package com.smsrelay.api

import com.google.gson.annotations.SerializedName
import okhttp3.ResponseBody
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.Header
import retrofit2.http.POST

data class LoginRequest(
    @SerializedName("master_password") val masterPassword: String,
)

data class LoginResponse(
    @SerializedName("access_token") val accessToken: String,
    @SerializedName("token_type") val tokenType: String,
)

data class DeviceAuthRequest(
    @SerializedName("master_password") val masterPassword: String,
    @SerializedName("device_name") val deviceName: String,
    @SerializedName("device_client_id") val deviceClientId: String,
)

data class DeviceAuthResponse(
    @SerializedName("device_token") val deviceToken: String,
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("token_type") val tokenType: String,
)

data class InboundRequest(
    val sender: String,
    val body: String,
    @SerializedName("received_at") val receivedAt: String,
    @SerializedName("device_name") val deviceName: String,
    @SerializedName("client_message_id") val clientMessageId: String,
)

data class InboundResponse(
    val id: String,
    @SerializedName("received_at") val receivedAt: String,
    @SerializedName("created_at") val createdAt: String,
)

interface SmsRelayApi {
    @POST("api/v1/auth/login")
    suspend fun login(@Body request: LoginRequest): LoginResponse

    @POST("api/v1/auth/device")
    suspend fun authDevice(@Body request: DeviceAuthRequest): DeviceAuthResponse

    @POST("api/v1/messages/inbound")
    suspend fun uploadMessage(
        @Header("Authorization") authorization: String,
        @Body message: InboundRequest,
    ): Response<ResponseBody>

    @POST("api/v1/devices/heartbeat")
    suspend fun heartbeat(
        @Header("Authorization") authorization: String,
    ): Response<ResponseBody>
}
