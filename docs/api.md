# API 规范

Base URL: `/api/v1`

## 鉴权

### 用户鉴权（Web 控制台）

```
Authorization: Bearer <user_jwt>
```

JWT payload 示例：

```json
{ "sub": "<user_id>", "typ": "user", "exp": 1710000000 }
```

### 设备鉴权（Android）

```
Authorization: Bearer <device_jwt>
```

JWT payload 示例：

```json
{ "sub": "<user_id>", "typ": "device", "device_id": "<device_id>", "exp": 1710000000 }
```

Device Token 由 `POST /auth/device` 用主密码换取，有效期 365 天。

---

## Auth

### POST /auth/register

一键注册，无需邮箱。

**Request:** 无 body

**Response 201:**

```json
{
  "access_token": "eyJ...",
  "token_type": "bearer",
  "master_password": "32位随机主密码，仅返回一次"
}
```

### POST /auth/login

主密码登录。**受 IP 限速保护**（见 [security.md](./security.md)）。

**Request:**

```json
{ "master_password": "..." }
```

**Response 200:**

```json
{
  "access_token": "eyJ...",
  "token_type": "bearer"
}
```

**Errors:** 401 凭证无效 · 429 请求过多

### POST /auth/device

Android 用主密码换取 Device Token。**受 IP 限速保护**。

**Request:**

```json
{
  "master_password": "...",
  "device_name": "Pixel 7",
  "device_client_id": "可选，App 端稳定 ID"
}
```

**Response 200:**

```json
{
  "device_token": "eyJ...",
  "device_id": "uuid",
  "token_type": "device"
}
```

---

## Messages

### POST /messages/inbound

Android 上报短信。需要 Device JWT。

**Request:**

```json
{
  "sender": "+8613800138000",
  "body": "您的验证码是 123456",
  "received_at": "2026-07-05T12:00:00+08:00",
  "device_name": "Android",
  "client_message_id": "可选，幂等 ID"
}
```

**Response 201:** 解密后的消息视图（id、sender、body、received_at、created_at）

**服务端行为：**

1. 向所有已启用的 Telegram destinations 转发
2. 加密 sender/body 入库
3. 写 forward_logs
4. SSE 推送 Web 客户端

### GET /messages

短信历史。需要 User JWT。

**Query:** `limit`（默认 50）· `offset`

### GET /messages/stream

SSE 实时推送。需要 User JWT。

事件：`connected` · `message`（新短信 JSON）

---

## Destinations

### GET /destinations

列出通知渠道（不含 bot_token）。

### POST /destinations

创建 Telegram 渠道。config 服务端加密存储。

**Request:**

```json
{
  "name": "我的 Telegram",
  "platform": "telegram",
  "config": {
    "bot_token": "123456:ABC...",
    "chat_id": "-1001234567890"
  }
}
```

### POST /destinations/telegram/link

启动 Telegram Bot 链接绑定流程（OAuth 式 deep link）。

### GET /destinations/telegram/link/:id

查询链接绑定状态。

### PATCH /destinations/{id}

更新 name、enabled、config。

### DELETE /destinations/{id}

删除渠道。

### GET /destinations/{id}/avatar

获取 Telegram 对话头像（代理 Bot API）。

---

## Devices

### GET /devices

列出当前用户的设备（id、name、last_seen_at、online、created_at）。

`online` 由服务端根据 `last_seen_at` 计算：`last_seen_at` 距今 **小于 30 分钟** 视为在线。该阈值对齐 Android 客户端心跳策略——Android 15+ 约 15 分钟 WorkManager 心跳，Android 14 及以下约 5 分钟前台服务心跳；短信上报也会刷新 `last_seen_at`。

```json
[
  {
    "id": "uuid",
    "name": "Pixel 8",
    "last_seen_at": "2026-07-06T12:00:00Z",
    "online": true,
    "created_at": "2026-07-01T08:00:00Z"
  }
]
```

### POST /devices/heartbeat

Device JWT 心跳，更新 `last_seen_at`。

---

## Health

### GET /health

```json
{ "status": "ok" }
```

---

## 错误格式

```json
{ "detail": "错误描述" }
```

| HTTP | 场景 |
|------|------|
| 400 | 参数无效 |
| 401 | 鉴权失败 |
| 403 | Token 类型不匹配 |
| 404 | 资源不存在 |
| 429 | 认证限速 |
| 500 | 服务器错误 |

---

## 使用示例

```bash
# 注册
curl -s -X POST http://localhost:8080/api/v1/auth/register | jq .

# 登录
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"master_password":"YOUR_PASSWORD"}' | jq -r .access_token)

# 设备 Token
DEVICE=$(curl -s -X POST http://localhost:8080/api/v1/auth/device \
  -H "Content-Type: application/json" \
  -d '{"master_password":"YOUR_PASSWORD","device_name":"测试机"}')
DEVICE_TOKEN=$(echo $DEVICE | jq -r .device_token)

# 上报短信
curl -s -X POST http://localhost:8080/api/v1/messages/inbound \
  -H "Authorization: Bearer $DEVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sender":"10086","body":"验证码 123456"}'

# 查看历史
curl -s "http://localhost:8080/api/v1/messages?limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 实现状态

| 端点 | 状态 |
|------|------|
| POST /auth/register | ✅ |
| POST /auth/login | ✅ |
| POST /auth/device | ✅ |
| POST /messages/inbound | ✅ |
| GET /messages | ✅ |
| GET /messages/stream | ✅ |
| GET/POST/PATCH/DELETE /destinations | ✅ |
| POST/GET /destinations/telegram/link | ✅ |
| GET /devices | ✅ |
| POST /devices/heartbeat | ✅ |
| GET /health | ✅ |
| 转发规则 API | ⬜ 未实现 |
