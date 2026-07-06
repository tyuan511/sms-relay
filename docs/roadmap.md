# 开发进度与计划

## 当前进度

### Server（Go + Fiber）

| 模块 | 状态 |
|------|------|
| SQLite + goose 迁移 + sqlc | ✅ |
| AES-256-GCM 字段加密 | ✅ |
| 主密码注册/登录 | ✅ |
| Device JWT 认证 | ✅ |
| 认证 IP 限速 + 失败退避 | ✅ |
| HMAC 密码指纹 + PASSWORD_PEPPER | ✅ |
| 可信代理 Client IP 解析 | ✅ |
| 短信上报 + 幂等 + 列表 | ✅ |
| Telegram 转发 + 链接绑定 | ✅ |
| SSE 实时推送 | ✅ |
| 通知渠道 CRUD | ✅ |
| 设备列表 + 心跳 | ✅ |

### Web（React + Vite）

| 模块 | 状态 |
|------|------|
| 一键注册 + 主密码展示 | ✅ |
| 主密码登录 | ✅ |
| 短信列表 + SSE 刷新 | ✅ |
| Telegram 通知配置 | ✅ |
| 设备列表 | ✅ |

### Android（Kotlin）

| 模块 | 状态 |
|------|------|
| 设置页（服务器 + 主密码） | ✅ |
| Device Token 认证 | ✅ |
| SmsReceiver + UploadSmsWorker | ✅ |
| 前台服务 + 开机自启 | ✅ |
| 离线队列（Room） | ✅ |

### 部署 / 运维

| 模块 | 状态 |
|------|------|
| Docker Compose 本地/生产 | ✅ |
| Web 容器内 nginx 反代 | ✅ |
| 宿主机 nginx 示例（安全 XFF） | ✅ |
| Docker Hub 镜像 + deploy.sh | ✅ |

---

## 技术栈

- **后端**：Go + Fiber + SQLite + sqlc + goose
- **鉴权**：32 位主密码 + JWT（User / Device 双 token）
- **前端**：React + Vite
- **Android**：Kotlin + Retrofit + WorkManager + Room

原 FastAPI + PostgreSQL + 邮箱注册 + device_secret 方案已废弃，文档已同步更新。

---

## Phase 4 — 功能增强（待做）

- [ ] 转发规则（关键词/发件人）
- [ ] Webhook / Email 转发
- [ ] 转发失败重试 Dashboard
- [ ] DEK 密钥轮换工具

---

## Phase 5 — 生产化（待做）

- [ ] CI/CD 流水线
- [ ] Android APK 签名与分发页
- [ ] 短信数据保留策略 / 自动清理

---

## 已完成的安全项

- [x] 认证端点应用层限速
- [x] 边缘 nginx 覆盖 X-Forwarded-For
- [x] 内层 nginx + 应用侧可信代理 IP 解析
- [x] PASSWORD_PEPPER 独立于 JWT_SECRET
