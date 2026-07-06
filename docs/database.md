# 数据库设计

SQLite（WAL 模式），迁移由 goose 管理，查询由 sqlc 生成。

## ER 关系

```
users ──┬── devices
        ├── destinations ──┐
        └── inbound_messages ── forward_logs
```

当前**无** `forward_rules` / `rule_destinations` 表；所有启用的 Telegram 渠道均接收每条短信。

## 表结构

### users

| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | UUID |
| password_hash | TEXT | bcrypt(主密码) |
| password_fingerprint | TEXT UNIQUE | HMAC-SHA256(主密码, PASSWORD_PEPPER) |
| created_at | DATETIME | |

### devices

| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | UUID |
| user_id | TEXT FK → users | |
| name | TEXT | 如「Android」「备用机」 |
| client_id | TEXT NULL | App 端稳定设备标识，用于去重复用 |
| last_seen_at | DATETIME NULL | 最后上报/心跳时间 |
| created_at | DATETIME | |

唯一约束：`(user_id, client_id)` WHERE client_id IS NOT NULL

### destinations

| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | |
| user_id | TEXT FK → users | |
| name | TEXT | 显示名称 |
| platform | TEXT | 当前仅 `telegram` |
| config_nonce | BLOB | AES-GCM nonce |
| config_enc | BLOB | 加密的 config JSON |
| key_version | INTEGER | 加密密钥版本，默认 1 |
| enabled | INTEGER | 1/0 |
| created_at | DATETIME | |

**config 明文结构（加密前）：**

```json
{
  "bot_token": "123456:ABC...",
  "chat_id": "-100xxx",
  "bot_username": "MyBot"
}
```

### inbound_messages

| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | |
| user_id | TEXT FK → users | |
| device_id | TEXT FK → devices NULL | |
| client_message_id | TEXT NULL | App 端幂等 ID |
| sender_nonce / sender_enc | BLOB | 加密发件人 |
| body_nonce / body_enc | BLOB | 加密正文 |
| key_version | INTEGER | |
| received_at | DATETIME | 手机收到时间 |
| created_at | DATETIME | 服务端入库时间 |

唯一索引：`(user_id, device_id, client_message_id)` WHERE client_message_id IS NOT NULL

### forward_logs

| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | |
| message_id | TEXT FK → inbound_messages | |
| destination_id | TEXT FK → destinations | |
| status | TEXT | success / failed |
| error | TEXT NULL | 失败原因（不含短信正文） |
| created_at | DATETIME | |

唯一索引：`(message_id, destination_id)`

## 索引

见 [`server/db/schema.sql`](../server/db/schema.sql)：

- `idx_messages_user_received` — 按用户分页查短信
- `idx_messages_device` — 按设备过滤
- `idx_forward_logs_message` — 查转发记录
- `idx_destinations_user` — 用户渠道列表

## 迁移

服务启动时自动执行 goose 迁移（`server/db/migrations/`）。

手动管理：

```bash
cd server
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir db/migrations sqlite3 ./data/smsrelay.db status
```

Schema 源文件：[`server/db/schema.sql`](../server/db/schema.sql)
