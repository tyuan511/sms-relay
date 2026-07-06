# 数据库加密方案

## 设计目标

| 目标 | 方案 |
|------|------|
| 数据库不存明文短信 | AES-256-GCM 字段级加密 |
| Telegram Token 不裸存 | 加密 `destinations.config` |
| 服务端能转发 | 内存中处理明文，处理后丢弃 |
| 拖库无法读短信 | DEK 不在数据库中 |
| 拖库难以离线猜主密码 | HMAC 指纹需 `PASSWORD_PEPPER`（见 [security.md](./security.md)） |

这是 **Encryption at Rest（静态加密）**，不是端到端加密。字段加密密钥由服务端持有。

## 密钥体系

```
环境变量 DATABASE_ENCRYPTION_KEY（32 字节，Base64）
       │
       ▼
  FieldEncryptor (AES-256-GCM)
       │
       ├── inbound_messages.sender
       ├── inbound_messages.body
       └── destinations.config

环境变量 PASSWORD_PEPPER（独立密钥）
       │
       ▼
  HMAC-SHA256(主密码, pepper) → users.password_fingerprint
```

主密码本身存 bcrypt 哈希；指纹仅用于登录索引，详见 [security.md](./security.md)。

### 密钥生成

```bash
openssl rand -base64 32
```

### 密钥存放

| 环境 | 做法 |
|------|------|
| 开发 | 项目根 `.env` |
| 生产 | Docker 环境变量 / `deploy.sh` 写入 `.env` |
| 禁止 | 写入 Git、写入数据库 |

## 加密算法

- **算法**：AES-256-GCM
- **Nonce**：每条记录 12 字节随机，不可复用
- **AAD**：绑定上下文，防串数据

| 字段 | AAD |
|------|-----|
| 短信 sender | `sender:{message_id}` |
| 短信 body | `body:{message_id}` |
| 目标配置 | `config:{destination_id}` |

## 加密字段一览

| 表 | 明文字段 | 存储 |
|----|----------|------|
| `inbound_messages` | sender | nonce + ciphertext |
| `inbound_messages` | body | nonce + ciphertext |
| `destinations` | config JSON | nonce + ciphertext |

### 保持明文

- `users.password_hash`（bcrypt，不可逆）
- `users.password_fingerprint`（HMAC  hex，不可逆猜解需 pepper）
- 时间戳、外键、platform、enabled 等元数据

## 代码实现

核心：`server/internal/crypto/crypto.go`

```go
type FieldEncryptor struct { ... }

func (e *FieldEncryptor) Encrypt(plaintext, aad string) (nonce, ciphertext []byte, err error)
func (e *FieldEncryptor) Decrypt(nonce, ciphertext []byte, aad string) (string, error)

func PasswordFingerprint(password, pepper string) string  // HMAC-SHA256
func LegacyPasswordFingerprint(password string) string    // 迁移用 SHA-256
```

调用链：

| 操作 | 位置 |
|------|------|
| 短信入库加密 | `services/message.go` |
| 短信列表解密 | `services/message.go` |
| 渠道 config 加解密 | `handlers/handlers.go` · `services/` |

## 数据流

```
App ──HTTPS──► 服务端内存（明文）
                    │
                    ├──► Telegram 转发（明文，即时丢弃）
                    │
                    └──► AES-GCM encrypt ──► SQLite（密文）

Web 查看 ◄── decrypt ◄── SQLite（密文）
```

## 密钥轮换（后续）

`key_version` 字段已预留。轮换 DEK 需后台任务 re-encrypt 全部密文字段。

`PASSWORD_PEPPER` 与 `JWT_SECRET` 轮换策略见 [security.md](./security.md)。

## 与端到端加密对比

| | 静态加密（当前） | 端到端 |
|--|-----------------|--------|
| 密钥持有者 | 服务端 | 用户 |
| 服务端能否转发 | 能 | 不能（除非解锁） |
| 拖库安全 | 高（+ HMAC pepper） | 高 |
| 复杂度 | 低 | 高 |
