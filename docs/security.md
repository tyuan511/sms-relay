# 安全设计

## 密钥体系

系统使用**三把独立密钥**，职责不可互换：

```
┌─────────────────────────┐
│ DATABASE_ENCRYPTION_KEY │  AES-256-GCM 字段加密（短信、Telegram Token）
└─────────────────────────┘

┌─────────────────────────┐
│ PASSWORD_PEPPER         │  HMAC-SHA256 密码指纹（登录索引，防拖库离线猜解）
└─────────────────────────┘  ⚠ 稳定密钥，丢失或变更会导致无法登录

┌─────────────────────────┐
│ JWT_SECRET              │  User / Device JWT 签名（可按需轮换）
└─────────────────────────┘
```

| 密钥 | 轮换影响 |
|------|----------|
| `DATABASE_ENCRYPTION_KEY` | 需 re-encrypt 全部密文字段 |
| `PASSWORD_PEPPER` | 已升级指纹的用户无法登录（需保留备份值） |
| `JWT_SECRET` | 旧 JWT 失效，用户/设备重新登录即可 |

### 密码存储

```
注册/登录主密码
    ├── password_hash      = bcrypt(密码)
    └── password_fingerprint = HMAC-SHA256(密码, PASSWORD_PEPPER)
                               （用于 GetUserByFingerprint 索引）
```

- 在线验证走 bcrypt，指纹仅用于 O(1) 用户查找
- 裸 SHA-256 指纹已废弃；登录时若命中旧格式会自动升级为 HMAC
- 若 `PASSWORD_PEPPER` 与历史 `JWT_SECRET` 不同，首次登录会尝试 JWT pepper 作为过渡并升级

### 密钥生成

```bash
openssl rand -base64 32
```

`.env` 中三项密钥均建议用独立随机值。生产首次部署可由 `deploy.sh` / `scripts/deploy.sh` 自动生成。

## 认证接口防护

`/api/v1/auth/login` 与 `/api/v1/auth/device` 挂载应用层限速中间件：

| 规则 | 值 |
|------|-----|
| 失败计数窗口 | 滑动 1 分钟 |
| 窗口内最大失败次数 | 10 次 / IP |
| 连续失败退避 | 第 3 次起指数退避（2s → 4s → …，上限 15 分钟） |
| 成功登录 | 清除该 IP 全部限速状态 |
| 超限响应 | HTTP 429 + `Retry-After` |

实现：`server/internal/middleware/auth_ratelimit.go`

## 客户端 IP 信任链

限速与审计依赖准确的客户端 IP。分三层防护：

### 1. 宿主机 Nginx（边缘）

**必须**覆盖客户端传入的转发头，写入真实连接 IP：

```nginx
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $remote_addr;
```

**禁止**使用 `$proxy_add_x_forwarded_for`（会保留伪造值）。

可选：对 auth 路径叠加 `limit_req`（见 `deploy/nginx.example.conf`）。

### 2. Web 容器 Nginx（内层）

[`web/nginx.conf`](../web/nginx.conf) 使用 `geo` 判断对端：

| 对端 | 行为 |
|------|------|
| 127.0.0.0/8、RFC1918 私网 | 透传边缘写入的 `X-Forwarded-For` |
| 公网直连 | 使用 `$remote_addr`，忽略请求自带 XFF |

避免开发环境 web 端口直接暴露时，攻击者伪造 IP 绕过限速。

### 3. Go 应用

[`ClientIPResolver`](../server/internal/middleware/clientip.go) 逻辑与内层 nginx 一致：

- 仅当 TCP 对端落在 `TRUSTED_PROXY_CIDRS`（默认 RFC1918 + loopback）时读取 `X-Forwarded-For` / `X-Real-IP`
- 否则使用 `c.IP()`

## JWT 载荷

```json
{
  "sub": "<user_id>",
  "typ": "user | device",
  "device_id": "<仅 device token>",
  "exp": 1234567890
}
```

- User JWT：控制台 API、SSE
- Device JWT：短信上报、设备心跳；服务端校验 `device_id` 归属

## 日志规范

**禁止**记录：

- 解密后的短信 `sender` / `body`
- Telegram `bot_token`
- 主密码明文

**允许**记录：

- `user_id`、`device_id`、`message_id`
- 转发 `status`、错误摘要（不含短信内容）
- 认证失败原因（不含密码）

## 已有环境升级

从旧版（无 `PASSWORD_PEPPER`）升级时：

1. 在 `.env` 添加 `PASSWORD_PEPPER=<当前 JWT_SECRET>`（若指纹已升级为 HMAC-JWT）
2. 或留空由 `deploy.sh` 自动生成（依赖登录时 JWT pepper 过渡 fallback）
3. 备份 `.env` 后重新部署
4. 同步更新宿主机 nginx，改用 `$remote_addr` 写入 XFF

## 威胁模型摘要

| 威胁 | 缓解 |
|------|------|
| 在线爆破主密码 | 62³² 空间 + 认证 IP 限速/退避 |
| 伪造 X-Forwarded-For 绕过限速 | 边缘覆盖 XFF + 应用侧可信代理校验 |
| 数据库拖库 | 字段 AES-GCM 加密；指纹 HMAC 需 pepper 才能离线验证 |
| JWT 泄露 | 短 TTL（User 7d / Device 365d）；可轮换 JWT_SECRET |
| 中间人 | 生产 HTTPS（TLS 在宿主机 nginx 终止） |
