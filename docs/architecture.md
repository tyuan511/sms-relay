# 系统架构

## 概述

SMS Relay 将 Android 手机收到的短信加密存储，并按用户配置转发到 Telegram。Web 控制台用于注册账号、配置通知渠道、查看历史。

当前实现：**Go + Fiber 后端**、**SQLite 持久化**、**React 控制台**、**Kotlin Android 客户端**。所有短信内容与 Telegram Bot Token 在数据库中以 AES-256-GCM 密文存储。

## 部署拓扑

### 生产环境（推荐）

公网流量经**宿主机 Nginx** 进入 Docker，Web 与 API 共用单域名：

```
Internet
    │  HTTPS
    ▼
┌─────────────────────────────────────────────────────────────┐
│  宿主机 Nginx（TLS 终止、IP 限速、覆盖 X-Forwarded-For）        │
│  proxy_set_header X-Forwarded-For $remote_addr              │
└───────────────────────────┬─────────────────────────────────┘
                            │ 127.0.0.1:WEB_PORT
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Docker: web 容器（nginx:alpine + 静态前端）                   │
│  · 托管 React 构建产物                                         │
│  · /api/ → 反代 server:8080                                 │
│  · 可信代理才透传 XFF，直连公网时用 $remote_addr               │
└───────────────────────────┬─────────────────────────────────┘
                            │ Docker 内网
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Docker: server 容器（Go Fiber :8080，不映射宿主机）           │
│  · SQLite（WAL 模式，volume 持久化）                           │
│  · 认证限速 / 加密 / Telegram 转发 / SSE                      │
└─────────────────────────────────────────────────────────────┘
```

参考配置：[`deploy/nginx.example.conf`](../deploy/nginx.example.conf)、[`web/nginx.conf`](../web/nginx.conf)、[`docker-compose.prod.yml`](../docker-compose.prod.yml)

### 本地 Docker 开发

[`docker-compose.yml`](../docker-compose.yml) 将 **web** 与 **server** 分别映射到宿主机端口（默认 5173 / 8080）。web 容器可能直接暴露于局域网，内层 nginx 对公网直连不信任客户端伪造的 `X-Forwarded-For`。

### 纯本地开发

Vite dev server（`:5173`）将 `/api` 代理到 Go 进程（`:8080`），无 nginx 层；应用侧对转发头采用默认可信代理 CIDR，直连时回退 `c.IP()`。

## 逻辑架构

```
┌──────────────┐   HTTPS    ┌──────────────────────────────────────┐
│ Android App  │──────────►│           Go Fiber Server               │
│ 短信采集/队列  │ Device JWT│                                      │
└──────────────┘           │  ┌────────────┐    ┌─────────────────┐  │
                           │  │ HTTP 路由   │───►│ MessageService  │  │
┌──────────────┐   HTTPS    │  └────────────┘    │ 入库·转发·SSE   │  │
│ Web 控制台    │──────────►│        │           └────────┬────────┘  │
│ React + Vite │ User JWT  │        │                    │           │
└──────────────┘           │  ┌─────▼─────┐   ┌─────────▼────────┐  │
                           │  │ 鉴权中间件  │   │ FieldEncryptor   │  │
                           │  │ JWT/Device │   │ AES-256-GCM      │  │
                           │  │ 认证限速    │   └─────────┬────────┘  │
                           │  └───────────┘             │           │
                           └──────────────────────────┼───────────┘
                                                        │
                        ┌───────────────────────────────┼───────────────┐
                        ▼                               ▼               ▼
                 ┌─────────────┐              ┌──────────────┐  ┌───────────┐
                 │ SQLite WAL  │              │ Telegram API │  │ SSE Hub   │
                 │ (密文)       │              │              │  │ (内存)    │
                 └─────────────┘              └──────────────┘  └───────────┘
```

## 核心数据流

### 1. 用户注册（Web）

```
Web POST /auth/register
  → 服务端 crypto/rand 生成 32 位主密码（大小写字母+数字）
  → bcrypt 哈希存 password_hash
  → HMAC-SHA256(密码, PASSWORD_PEPPER) 存 password_fingerprint（登录索引）
  → 签发 User JWT（7 天）
  → 响应中一次性返回 master_password + access_token
```

### 2. 设备认证（Android）

```
App POST /auth/device { master_password, device_name, device_client_id? }
  → 认证限速中间件（按客户端 IP）
  → 指纹查用户 + bcrypt 校验
  → 创建或复用 devices 记录
  → 签发 Device JWT（365 天，含 device_id）
  → 后续上报携带 Authorization: Bearer <device_token>
```

设备**无需**在 Web 预先注册；首次上报时也可由服务端自动创建设备记录（兼容旧路径）。

### 3. 短信上报与转发

```
Android 收到 SMS_RECEIVED
  → Room 本地队列（离线容错）
  → WorkManager POST /messages/inbound（Device JWT）
  → MessageService.ProcessInbound:
       a. 校验 device 归属
       b. 幂等检查（user_id + device_id + client_message_id）
       c. 对 enabled 的 Telegram destinations 逐条转发
       d. AES-GCM 加密 sender/body 后写入 inbound_messages
       e. 写 forward_logs（不含短信正文）
       f. SSE Hub 推送 Web 客户端
  → 201
```

明文仅在服务端内存中短暂存在，不写应用日志、不落盘明文。

### 4. Web 实时刷新

```
Web GET /messages/stream（User JWT）
  → SSE 长连接
  → 新短信入库后 Hub 广播 event: message
  → 前端刷新列表
```

## 鉴权模型

| 角色 | Token 类型 | 获取方式 | 用途 |
|------|-----------|----------|------|
| Web 用户 | User JWT (`typ=user`) | `/auth/register` 或 `/auth/login` | 控制台 API、SSE |
| Android 设备 | Device JWT (`typ=device`, 含 `device_id`) | `/auth/device` | 短信上报、心跳 |

主密码：
- 32 位随机字符串，**仅注册时展示一次**
- 数据库存 `bcrypt` 哈希 + `password_fingerprint`（HMAC 索引，见 [security.md](./security.md)）
- 在线猜解空间约 62³²；认证接口有 IP 限速与失败退避

## 项目结构

```
sms-relay/
├── server/                      # Go 后端
│   ├── cmd/server/main.go       # 入口、路由注册
│   ├── db/
│   │   ├── migrations/          # goose SQL 迁移
│   │   ├── queries/             # sqlc 查询定义
│   │   └── schema.sql
│   └── internal/
│       ├── auth/                # JWT、主密码生成/校验
│       ├── config/              # 环境变量
│       ├── crypto/              # AES-GCM、密码指纹
│       ├── db/                  # sqlc 生成代码
│       ├── handlers/            # HTTP 处理器
│       ├── middleware/          # JWT、认证限速、Client IP
│       ├── migrate/             # goose 封装
│       ├── services/            # 短信入库、Telegram 转发
│       ├── sse/                 # SSE Hub
│       └── telegram/            # Bot API、链接绑定
├── web/                         # React + Vite 控制台
│   ├── nginx.conf               # 容器内 /api 反代
│   └── src/
├── android/                     # Kotlin 客户端
├── deploy/
│   └── nginx.example.conf       # 宿主机反代示例
├── docs/                        # 设计文档（本目录）
├── scripts/deploy.sh            # 本地 Docker 部署
└── deploy.sh                    # 生产镜像部署
```

## 环境变量（服务端）

| 变量 | 必填 | 说明 |
|------|------|------|
| `DATABASE_ENCRYPTION_KEY` | 是 | 32 字节 Base64，字段加密 DEK |
| `PASSWORD_PEPPER` | 是 | 密码指纹 HMAC 密钥，**独立于 JWT，勿随 JWT 轮换** |
| `JWT_SECRET` | 是 | JWT 签名密钥，可按需轮换 |
| `DATABASE_PATH` | 否 | SQLite 路径，默认 `./data/smsrelay.db` |
| `CORS_ORIGIN` | 否 | Web 来源，生产与 `PUBLIC_URL` 一致 |
| `TRUSTED_PROXY_CIDRS` | 否 | 信任转发头的代理 CIDR，默认 RFC1918 + loopback |

完整安全说明见 [security.md](./security.md)。

## 转发模型

当前版本：**所有已启用的 Telegram 通知渠道均会收到每条短信**（无关键词/发件人规则引擎）。`forward_logs` 记录每条消息对各 destination 的转发结果。

| 平台 | 状态 |
|------|------|
| Telegram | ✅ Bot Token + Chat ID，支持链接绑定流程 |
| Webhook / Email | ⬜ 未实现 |

## 相关文档

| 文档 | 内容 |
|------|------|
| [security.md](./security.md) | 密钥体系、认证限速、IP 信任链 |
| [encryption.md](./encryption.md) | AES-GCM 字段加密 |
| [database.md](./database.md) | SQLite 表结构 |
| [api.md](./api.md) | REST API 规范 |
| [android.md](./android.md) | Android 客户端 |
| [web.md](./web.md) | Web 控制台 |
