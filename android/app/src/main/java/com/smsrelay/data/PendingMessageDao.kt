package com.smsrelay.data

import androidx.paging.PagingSource
import androidx.room.Dao
import androidx.room.Insert
import androidx.room.Query
import androidx.room.Update
import kotlinx.coroutines.flow.Flow

@Dao
interface PendingMessageDao {
    @Insert
    suspend fun insert(message: PendingMessage): Long

    @Query("SELECT * FROM pending_messages WHERE id = :id LIMIT 1")
    suspend fun getById(id: Long): PendingMessage?

    @Query("SELECT * FROM pending_messages WHERE status = :status ORDER BY createdAt ASC")
    suspend fun listByStatus(status: String): List<PendingMessage>

    @Query("SELECT COUNT(*) FROM pending_messages WHERE status = :status")
    suspend fun countByStatus(status: String): Int

    @Query("SELECT COUNT(*) FROM pending_messages")
    suspend fun countAll(): Int

    @Query("SELECT * FROM pending_messages ORDER BY createdAt DESC")
    fun observeAll(): Flow<List<PendingMessage>>

    @Query("SELECT * FROM pending_messages ORDER BY createdAt DESC")
    fun pagingSource(): PagingSource<Int, PendingMessage>

    @Update
    suspend fun update(message: PendingMessage)
}
