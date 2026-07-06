# Web 管理后台

## 职责

Web 控制台供用户：

1. 一键注册 / 主密码登录
2. 查看短信历史（SSE 实时刷新）
3. 配置 Telegram 通知渠道
4. 查看已注册 Android 设备

当前**无**转发规则配置页；所有启用的 Telegram 渠道均接收每条短信。

## 技术栈

| 组件 | 选型 |
|------|------|
| 框架 | React 18 + Vite |
| 路由 | React Router |
| 样式 | Tailwind CSS（Vercel 风格设计系统，见根目录 `DESIGN.md`） |
| 请求 | fetch |
| 包管理 | pnpm workspace |
| 部署 | Docker 内 nginx 静态托管；开发时 Vite 代理 `/api` |

## 页面结构

```
/                     → 注册 / 登录
/messages             → 短信列表（SSE 刷新）
/settings             → Telegram 通知渠道
/devices              → 设备列表
```

## 鉴权

- 注册：`POST /auth/register` → 展示 `master_password`（一次性）+ 存 User JWT
- 登录：`POST /auth/login` → User JWT 存 localStorage
- 请求头：`Authorization: Bearer <token>`
- Token 过期后跳转登录页

Android 设备**不在 Web 注册**；App 使用同一主密码调用 `/auth/device`。

## Telegram 配置

两种绑定方式：

1. **链接绑定**：填写 Bot Token → 服务端生成 deep link → 用户在 Telegram 点 Start
2. **手动填写**：直接输入 Bot Token + Chat ID

配置加密存储于服务端，Web 仅展示 bot_username、chat_id 等非敏感字段。

## 本地开发

在项目根目录安装 workspace 依赖并启动：

```bash
pnpm install
pnpm dev:web   # http://localhost:5173，/api 代理到 :8080
```

也可在 `web/` 子目录直接运行 `pnpm dev`（需已执行根目录 `pnpm install`）。

## 生产构建

```bash
pnpm build:web
```

产物由 [`web/Dockerfile`](../web/Dockerfile) 打入 nginx 镜像；API 由容器内 [`web/nginx.conf`](../web/nginx.conf) 反代至 `server:8080`。

## 相关文档

- 设计规范：[DESIGN.md](../DESIGN.md)
- API：[api.md](./api.md)
- 架构：[architecture.md](./architecture.md)
