# Android 客户端实现指南

## 职责

Android App 只做一件事：**监听短信，上报服务器**。不处理转发规则，不存储平台 Token。

```
收到短信 → 读取 sender/body → HTTPS POST → 完成
```

## 技术栈

| 组件 | 推荐 |
|------|------|
| 语言 | Kotlin |
| 网络 | Retrofit + OkHttp |
| 离线队列 | Room + WorkManager |
| 本地存储 | EncryptedSharedPreferences（存 Device Token） |
| 最低 SDK | 26 (Android 8.0) |
| 目标 SDK | 34+ |

## 权限

```xml
<uses-permission android:name="android.permission.RECEIVE_SMS" />
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
<uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
```

### Android 15+ 侧载限制

若 APK 侧载安装（非 Play Store），用户需手动：

1. 应用信息 → ⋮ → **允许受限设置**
2. 权限 → SMS → **允许**

App 首次启动应检测权限状态，引导用户完成上述步骤。

## 架构

```
SMS_RECEIVED (BroadcastReceiver)
    → 写入 Room 本地队列 (pending)
    → 前台服务短暂启动尝试即时转发
        → 成功：标记 done
        → 可重试失败：WorkManager 单条重试

WorkManager 兜底：
    → 网络恢复 / 定期扫描 pending
    → 手动「立即同步」
    → 设备心跳（15 分钟周期，不占用 FGS 配额）

用户侧：
    → 关闭电池优化 + 厂商自启动（小米/华为/OPPO 等）
```

短信接收不依赖 App 常驻进程；`SmsReceiver` 在系统广播里短任务入队即可。持续监听状态、设备在线心跳、即时转发走 **Foreground Service**（Android 8+ 必需）。

## 核心组件

### 1. SmsReceiver

监听 `SMS_RECEIVED`，只做两件事：写入本地队列、通知前台服务上传。不在 Receiver 里直接发网络请求（10 秒限制），也不并行 enqueue WorkManager（除非前台服务无法启动）。

### 2. SmsMonitorService（前台服务）

- 仅在收到短信需要上传时短暂启动，显示「正在转发短信」通知
- 上传完成后立即 `stopSelf()`，不常驻
- Android 15 实现 `onTimeout()`，超时后 fallback 到 WorkManager
- `BOOT_COMPLETED` 不启动此服务（Android 15 禁止从开机广播启动 dataSync FGS）

### 3. WorkManager 兜底

| Worker | 职责 |
|--------|------|
| `UploadSmsWorker` | 单条消息重试（前台服务失败后的 fallback） |
| `PendingSyncWorker` | 定期 / 网络恢复 / 手动同步扫描全部 pending |
| `HeartbeatWorker` | 设备在线心跳（15 分钟周期） |

### 4. 电池白名单

App 内提供「关闭电池优化」入口；国产 ROM 还需用户手动开启自启动、锁定后台等（见厂商适配清单）。

### 5. API 接口

```kotlin
interface SmsRelayApi {
    @POST("api/v1/auth/device")
    suspend fun authDevice(@Body req: DeviceAuthRequest): DeviceAuthResponse

    @POST("api/v1/messages/inbound")
    suspend fun uploadMessage(@Body message: MessageInbound): MessageResponse
}
```

首次或 Token 过期时，App 用主密码调用 `/auth/device` 获取 `device_token` 并本地缓存。

### 6. 开机自启

```xml
<receiver android:name=".BootReceiver" android:exported="true">
    <intent-filter>
        <action android:name="android.intent.action.BOOT_COMPLETED" />
    </intent-filter>
</receiver>
```

## 设置页

App 只需一个简单的配置界面：

| 设置项 | 说明 |
|--------|------|
| 服务器地址 | 如 `https://sms.example.com`（不含 `/api/v1`） |
| 主密码 | Web 注册时保存的 32 位密码 |
| 连接状态 | 上次成功上报时间 |

可选：
- 「测试连接」按钮：发送一条测试短信
- 权限检查与引导

## 离线队列

无网络时短信不能丢失，先落库再转发：

```
SmsReceiver → Room (pending) → SmsMonitorService 尝试上传
                                    ↓ 失败
                              WorkManager 重试 / 定期扫描
```

Room 表结构：

| 列 | 类型 |
|----|------|
| id | AUTO |
| sender | TEXT |
| body | TEXT |
| received_at | TEXT |
| status | TEXT (pending/done/failed) |
| retry_count | INT |

## 项目结构建议

```
android/
├── app/src/main/
│   ├── java/com/smsrelay/
│   │   ├── SmsRelayApp.kt
│   │   ├── receiver/
│   │   │   ├── SmsReceiver.kt
│   │   │   └── BootReceiver.kt
│   │   ├── worker/
│   │   │   └── UploadSmsWorker.kt
│   │   ├── service/
│   │   │   └── SmsMonitorService.kt
│   │   ├── api/
│   │   │   ├── SmsRelayApi.kt
│   │   │   └── DeviceAuthInterceptor.kt
│   │   ├── data/
│   │   │   ├── AppDatabase.kt
│   │   │   └── PendingMessage.kt
│   │   ├── ui/
│   │   │   └── SettingsActivity.kt
│   │   └── util/
│   │       └── Prefs.kt
│   ├── res/
│   └── AndroidManifest.xml
└── build.gradle.kts
```

## 分发

Google Play 不允许普通 App 使用 SMS 权限。分发方式：

- APK 侧载（官网下载）
- 企业内部 MDM 分发
- F-Droid（开源项目）

## 厂商适配清单

引导用户完成的设置（按厂商不同）：

| 厂商 | 设置项 |
|------|--------|
| 小米 | 自启动、电池优化无限制、锁定后台 |
| 华为 | 启动管理、电池优化 |
| OPPO/vivo | 自启动、后台运行 |
| 三星 | 电池优化例外 |
| 原生 Android | 电池优化例外、Android 15 受限设置 |

App 内应检测厂商并提供跳转引导。

## 安全

- Device Token 用 EncryptedSharedPreferences 存储
- 禁止 Log 输出短信 body 与主密码
- 服务器地址应使用 HTTPS
