package com.smsrelay.data

import androidx.room.Entity
import androidx.room.ColumnInfo
import androidx.room.PrimaryKey
import java.util.UUID

@Entity(tableName = "pending_messages")
data class PendingMessage(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val sender: String,
    val body: String,
    val receivedAt: String,
    @ColumnInfo(defaultValue = "")
    val clientMessageId: String = UUID.randomUUID().toString(),
    val status: String = STATUS_PENDING,
    val retryCount: Int = 0,
    val lastError: String? = null,
    val createdAt: Long = System.currentTimeMillis(),
) {
    companion object {
        const val STATUS_PENDING = "pending"
        const val STATUS_DONE = "done"
        const val STATUS_FAILED = "failed"
    }
}
