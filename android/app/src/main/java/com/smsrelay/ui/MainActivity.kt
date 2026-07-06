package com.smsrelay.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.widget.EditText
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import androidx.paging.LoadState
import androidx.paging.Pager
import androidx.paging.PagingConfig
import androidx.paging.cachedIn
import androidx.recyclerview.widget.LinearLayoutManager
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.smsrelay.R
import com.smsrelay.api.ApiClient
import com.smsrelay.api.DeviceAuthRequest
import com.smsrelay.api.InboundRequest
import com.smsrelay.data.AppDatabase
import com.smsrelay.data.PendingMessage
import com.smsrelay.data.Prefs
import com.smsrelay.databinding.ActivityMainBinding
import com.smsrelay.databinding.DialogSettingsBinding
import com.smsrelay.service.SmsMonitorService
import com.smsrelay.util.BatteryWhitelist
import com.smsrelay.util.ServerUrl
import com.smsrelay.util.SyncScheduler
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import retrofit2.HttpException
import java.io.IOException
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.UUID

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var prefs: Prefs
    private val messageAdapter = MessageAdapter()

    private var settingsDialogBinding: DialogSettingsBinding? = null

    private val permissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) {
        updatePermissionButton()
        if (prefs.isConfigured() && !hasSmsPermission()) {
            Toast.makeText(this, R.string.permission_required, Toast.LENGTH_LONG).show()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        prefs = Prefs(this)

        binding.serverUrlInput.setText(prefs.serverUrl)
        if (prefs.masterPassword.isNotBlank()) {
            binding.passwordInput.setText(prefs.masterPassword)
        }

        binding.messageList.layoutManager = LinearLayoutManager(this)
        binding.messageList.adapter = messageAdapter
        binding.messageList.setItemViewCacheSize(20)

        binding.saveButton.setOnClickListener { saveConfig() }
        binding.testButton.setOnClickListener { testConnection() }
        binding.permissionButton.setOnClickListener { requestPermissions() }
        binding.syncButton.setOnClickListener {
            SyncScheduler.enqueueImmediateSync(this)
            Toast.makeText(this, "已开始同步待上传短信", Toast.LENGTH_SHORT).show()
        }
        binding.batteryButton.setOnClickListener { requestBatteryWhitelist() }

        binding.toolbar.setOnMenuItemClickListener { item ->
            when (item.itemId) {
                R.id.action_settings -> {
                    showSettingsDialog()
                    true
                }
                else -> false
            }
        }

        observeMessages()
        refreshScreen()
        if (prefs.isConfigured()) {
            SmsMonitorService.startMonitoring(this)
        }
    }

    override fun onResume() {
        super.onResume()
        refreshScreen()
        if (prefs.isConfigured()) {
            updateBatteryButton()
        }
    }

    private fun refreshScreen() {
        val configured = prefs.isConfigured()
        binding.setupContainer.visibility = if (configured) View.GONE else View.VISIBLE
        binding.mainContainer.visibility = if (configured) View.VISIBLE else View.GONE

        if (configured) {
            updateStatus()
            updatePermissionButton()
            updateBatteryButton()
            if (!hasSmsPermission()) {
                requestPermissions()
            }
        } else {
            updatePermissionButton()
            binding.batteryButton.visibility = View.GONE
        }
    }

    private fun observeMessages() {
        val dao = AppDatabase.get(this).pendingMessageDao()
        val pagerFlow = Pager(
            config = PagingConfig(
                pageSize = 20,
                prefetchDistance = 5,
                enablePlaceholders = false,
            ),
            pagingSourceFactory = { dao.pagingSource() },
        ).flow.cachedIn(lifecycleScope)

        lifecycleScope.launch {
            pagerFlow.collectLatest { pagingData ->
                messageAdapter.submitData(pagingData)
            }
        }

        lifecycleScope.launch {
            messageAdapter.loadStateFlow.collectLatest { loadState ->
                val isEmpty = loadState.refresh is LoadState.NotLoading &&
                    messageAdapter.itemCount == 0
                binding.emptyState.visibility = if (isEmpty) View.VISIBLE else View.GONE
            }
        }
    }

    private fun showSettingsDialog() {
        val dialogBinding = DialogSettingsBinding.inflate(LayoutInflater.from(this))
        settingsDialogBinding = dialogBinding
        dialogBinding.dialogServerUrlInput.setText(prefs.serverUrl)
        dialogBinding.dialogPasswordInput.setText(prefs.masterPassword)

        val dialog = MaterialAlertDialogBuilder(this)
            .setTitle(R.string.settings_dialog_title)
            .setView(dialogBinding.root)
            .setNegativeButton(android.R.string.cancel, null)
            .setOnDismissListener { settingsDialogBinding = null }
            .create()

        dialogBinding.dialogSaveButton.setOnClickListener {
            saveConfig(
                urlInput = dialogBinding.dialogServerUrlInput,
                passwordInput = dialogBinding.dialogPasswordInput,
                onSuccess = { dialog.dismiss() },
            )
        }
        dialogBinding.dialogTestButton.setOnClickListener {
            testConnection(
                urlInput = dialogBinding.dialogServerUrlInput,
                passwordInput = dialogBinding.dialogPasswordInput,
            )
        }

        dialog.show()
    }

    private fun saveConfig(
        urlInput: EditText? = null,
        passwordInput: EditText? = null,
        onSuccess: (() -> Unit)? = null,
    ) {
        val urlField = urlInput ?: binding.serverUrlInput
        val passwordField = passwordInput ?: binding.passwordInput
        val fromDialog = urlInput != null

        val url = ServerUrl.normalize(urlField.text?.toString().orEmpty())
        val password = passwordField.text?.toString()?.trim().orEmpty()

        if (url.isBlank() || password.isBlank()) {
            Toast.makeText(this, "请填写服务器地址和主密码", Toast.LENGTH_SHORT).show()
            return
        }

        urlField.setText(url)
        setConfigButtonsEnabled(fromDialog, enabled = false)

        lifecycleScope.launch {
            try {
                val deviceName = android.os.Build.MODEL ?: "Android"
                val auth = withContext(Dispatchers.IO) {
                    ApiClient.create(url).authDevice(
                        DeviceAuthRequest(
                            masterPassword = password,
                            deviceName = deviceName,
                            deviceClientId = prefs.deviceClientId,
                        ),
                    )
                }

                prefs.serverUrl = url
                prefs.masterPassword = password
                prefs.deviceToken = auth.deviceToken
                prefs.deviceId = auth.deviceId

                Toast.makeText(this@MainActivity, R.string.config_saved, Toast.LENGTH_SHORT).show()
                refreshScreen()
                SyncScheduler.scheduleHeartbeat(this@MainActivity)
                SmsMonitorService.startMonitoring(this@MainActivity)
                SyncScheduler.enqueueImmediateSync(this@MainActivity)
                requestPermissions()
                onSuccess?.invoke()
            } catch (e: Exception) {
                Toast.makeText(this@MainActivity, connectionErrorMessage(e, url), Toast.LENGTH_LONG).show()
            } finally {
                setConfigButtonsEnabled(fromDialog, enabled = true)
            }
        }
    }

    private fun testConnection(
        urlInput: EditText? = null,
        passwordInput: EditText? = null,
    ) {
        val urlField = urlInput ?: binding.serverUrlInput
        val passwordField = passwordInput ?: binding.passwordInput
        val fromDialog = urlInput != null

        val url = ServerUrl.normalize(urlField.text?.toString().orEmpty())
        val password = passwordField.text?.toString()?.trim().orEmpty()
        if (url.isBlank() || password.isBlank()) {
            Toast.makeText(this, "请先填写配置", Toast.LENGTH_SHORT).show()
            return
        }

        urlField.setText(url)
        setConfigButtonsEnabled(fromDialog, enabled = false, testOnly = true)
        toast(getString(R.string.testing_connection))

        lifecycleScope.launch {
            try {
                val deviceName = android.os.Build.MODEL ?: "Android"
                val auth = withContext(Dispatchers.IO) {
                    ApiClient.create(url).authDevice(
                        DeviceAuthRequest(
                            masterPassword = password,
                            deviceName = deviceName,
                            deviceClientId = prefs.deviceClientId,
                        ),
                    )
                }
                val uploadResponse = withContext(Dispatchers.IO) {
                    ApiClient.create(url).uploadMessage(
                        "Bearer ${auth.deviceToken}",
                        InboundRequest(
                            sender = "SMS Relay",
                            body = "连接测试成功 ✓",
                            receivedAt = java.time.Instant.now().toString(),
                            deviceName = deviceName,
                            clientMessageId = "test-${UUID.randomUUID()}",
                        ),
                    )
                }
                if (!uploadResponse.isSuccessful) {
                    throw HttpException(uploadResponse)
                }
                showToast(getString(R.string.test_success), Toast.LENGTH_LONG)
            } catch (e: Exception) {
                showToast(connectionErrorMessage(e, url), Toast.LENGTH_LONG)
            } finally {
                setConfigButtonsEnabled(fromDialog, enabled = true, testOnly = true)
            }
        }
    }

    private fun toast(message: String, duration: Int = Toast.LENGTH_SHORT) {
        Toast.makeText(this, message, duration).show()
    }

    private suspend fun showToast(message: String, duration: Int = Toast.LENGTH_SHORT) {
        withContext(Dispatchers.Main) {
            toast(message, duration)
        }
    }

    private fun setConfigButtonsEnabled(fromDialog: Boolean, enabled: Boolean, testOnly: Boolean = false) {
        if (fromDialog) {
            val dialogBinding = settingsDialogBinding ?: return
            if (!testOnly) dialogBinding.dialogSaveButton.isEnabled = enabled
            dialogBinding.dialogTestButton.isEnabled = enabled
            return
        }
        if (!testOnly) binding.saveButton.isEnabled = enabled
        binding.testButton.isEnabled = enabled
    }

    private fun connectionErrorMessage(e: Exception, serverUrl: String): String {
        return when (e) {
            is HttpException -> when (e.code()) {
                401 -> "主密码错误，请使用在 $serverUrl 注册时保存的 32 位主密码"
                404 -> "接口不存在，请确认服务器已更新到最新版本"
                in 500..599 -> "服务器错误 (${e.code()})，请稍后重试"
                else -> "请求失败 (HTTP ${e.code()})"
            }
            is IOException -> "无法连接服务器，请检查地址和网络：${e.message ?: "网络错误"}"
            else -> getString(R.string.login_failed) + "：${e.message ?: e.javaClass.simpleName}"
        }
    }

    private fun updateStatus() {
        if (prefs.isConfigured()) {
            binding.statusText.text = getString(R.string.status_connected)
            binding.statusText.setTextColor(getColor(android.R.color.holo_green_light))
        } else {
            binding.statusText.text = getString(R.string.status_disconnected)
            binding.statusText.setTextColor(getColor(android.R.color.darker_gray))
        }

        val last = prefs.lastUploadAt
        binding.lastUploadText.text = if (last > 0) {
            val fmt = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault())
            getString(R.string.last_upload, fmt.format(Date(last)))
        } else {
            getString(R.string.last_upload, getString(R.string.never))
        }

        lifecycleScope.launch {
            val pending = withContext(Dispatchers.IO) {
                AppDatabase.get(this@MainActivity).pendingMessageDao()
                    .countByStatus(PendingMessage.STATUS_PENDING)
            }
            if (pending > 0) {
                binding.pendingText.visibility = View.VISIBLE
                binding.pendingText.text = getString(R.string.pending_queue, pending)
                binding.syncButton.visibility = View.VISIBLE
            } else {
                binding.pendingText.visibility = View.GONE
                binding.syncButton.visibility = View.GONE
            }
        }
    }

    private fun hasSmsPermission(): Boolean {
        return ContextCompat.checkSelfPermission(this, Manifest.permission.RECEIVE_SMS) ==
            PackageManager.PERMISSION_GRANTED
    }

    private fun requestPermissions() {
        val perms = mutableListOf(Manifest.permission.RECEIVE_SMS)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            perms.add(Manifest.permission.POST_NOTIFICATIONS)
        }
        permissionLauncher.launch(perms.toTypedArray())
    }

    private fun updatePermissionButton() {
        if (!::binding.isInitialized) return
        val hasSms = hasSmsPermission()
        binding.permissionButton.text = if (hasSms) {
            "短信权限已授予 ✓"
        } else {
            getString(R.string.grant_permission)
        }
        binding.permissionButton.isEnabled = !hasSms
        binding.permissionButton.visibility = if (hasSms) View.GONE else View.VISIBLE
    }

    private fun updateBatteryButton() {
        if (!::binding.isInitialized) return
        val whitelisted = BatteryWhitelist.isIgnoringOptimizations(this)
        binding.batteryButton.text = if (whitelisted) {
            getString(R.string.battery_whitelisted)
        } else {
            getString(R.string.battery_whitelist_action)
        }
        binding.batteryButton.isEnabled = !whitelisted
        binding.batteryButton.visibility = View.VISIBLE
    }

    private fun requestBatteryWhitelist() {
        if (BatteryWhitelist.isIgnoringOptimizations(this)) return
        try {
            startActivity(BatteryWhitelist.requestExemptionIntent(this))
        } catch (_: Exception) {
            startActivity(BatteryWhitelist.openSettingsIntent())
        }
    }
}
