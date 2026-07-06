package com.smsrelay.ui

import android.graphics.drawable.GradientDrawable
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.paging.PagingDataAdapter
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.RecyclerView
import com.smsrelay.R
import com.smsrelay.data.PendingMessage
import com.smsrelay.databinding.ItemMessageBinding
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class MessageAdapter : PagingDataAdapter<PendingMessage, MessageAdapter.ViewHolder>(DiffCallback) {

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): ViewHolder {
        val binding = ItemMessageBinding.inflate(LayoutInflater.from(parent.context), parent, false)
        return ViewHolder(binding)
    }

    override fun onBindViewHolder(holder: ViewHolder, position: Int) {
        getItem(position)?.let { holder.bind(it) }
    }

    class ViewHolder(
        private val binding: ItemMessageBinding,
    ) : RecyclerView.ViewHolder(binding.root) {

        private val timeFormat = SimpleDateFormat("MM-dd HH:mm", Locale.getDefault())

        fun bind(message: PendingMessage) {
            binding.senderText.text = message.sender
            binding.bodyText.text = message.body
            binding.timeText.text = formatTime(message)

            when (message.status) {
                PendingMessage.STATUS_DONE -> {
                    applyBadge(
                        binding.statusBadge,
                        binding.root.context.getString(R.string.status_forwarded),
                        0xFF34D399.toInt(),
                        0x3310B981,
                    )
                    binding.errorText.visibility = View.GONE
                }
                PendingMessage.STATUS_FAILED -> {
                    applyBadge(
                        binding.statusBadge,
                        binding.root.context.getString(R.string.status_failed),
                        0xFFF87171.toInt(),
                        0x33EF4444,
                    )
                    binding.errorText.visibility = if (message.lastError.isNullOrBlank()) {
                        View.GONE
                    } else {
                        View.VISIBLE
                    }
                    binding.errorText.text = message.lastError
                }
                else -> {
                    applyBadge(
                        binding.statusBadge,
                        binding.root.context.getString(R.string.status_pending),
                        0xFFFBBF24.toInt(),
                        0x33F59E0B,
                    )
                    binding.errorText.visibility = if (message.lastError.isNullOrBlank()) {
                        View.GONE
                    } else {
                        View.VISIBLE
                    }
                    binding.errorText.text = message.lastError
                }
            }
        }

        private fun formatTime(message: PendingMessage): String {
            val millis = runCatching {
                java.time.Instant.parse(message.receivedAt).toEpochMilli()
            }.getOrElse { message.createdAt }
            return timeFormat.format(Date(millis))
        }

        private fun applyBadge(view: android.widget.TextView, text: String, textColor: Int, bgColor: Int) {
            view.text = text
            view.setTextColor(textColor)
            val drawable = (view.background as? GradientDrawable) ?: GradientDrawable().also {
                it.cornerRadius = 999f
                view.background = it
            }
            drawable.setColor(bgColor)
        }
    }

    private object DiffCallback : DiffUtil.ItemCallback<PendingMessage>() {
        override fun areItemsTheSame(oldItem: PendingMessage, newItem: PendingMessage): Boolean {
            return oldItem.id == newItem.id
        }

        override fun areContentsTheSame(oldItem: PendingMessage, newItem: PendingMessage): Boolean {
            return oldItem == newItem
        }
    }
}
