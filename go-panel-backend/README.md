# Go 后端说明

本目录是当前项目默认使用的后端实现。

技术栈：

- Go
- SQLite
- 原生 `net/http`
- WebSocket

---

## 主要职责

该后端负责：

- 用户登录与权限控制
- 用户、节点、隧道、转发、限速规则管理
- SQLite 数据库存储
- 节点 WebSocket 管理
- `/flow/*` 流量上报接口
- 与前端 `/api/v1/*` 接口兼容

---

## 环境变量

支持的主要环境变量如下：

- `APP_ADDR`
  默认值：`:6365`

- `DATABASE_PATH`
  默认值：`data/flux-panel.db`

- `JWT_SECRET`
  JWT 签名密钥，建议手动设置

- `ADMIN_USERNAME`
  默认值：`admin_user`

- `ADMIN_PASSWORD`
  初始管理员密码

- `CORS_ALLOWED_ORIGINS`
  逗号分隔的允许跨域来源

- `LOGIN_RATE_LIMIT_PER_MINUTE`
  登录接口限速，默认 `12`

---

## 本地运行

进入目录：

```bash
cd go-panel-backend
```

安装依赖：

```bash
go mod tidy
```

运行：

```bash
go run .
```

Windows PowerShell 示例：

```powershell
$env:APP_ADDR="127.0.0.1:6365"
$env:DATABASE_PATH=".\data\flux-panel.db"
$env:JWT_SECRET="your-secret"
$env:ADMIN_USERNAME="admin_user"
$env:ADMIN_PASSWORD="your-password"
go run .
```

---

## Docker 构建

在仓库根目录使用：

```bash
docker compose build backend
```

或在当前目录单独构建：

```bash
docker build -t dlam-backend .
```

---

## 数据文件

源码运行时，数据库通常位于：

- `go-panel-backend/data/flux-panel.db`

SQLite 运行时还可能生成：

- `flux-panel.db-shm`
- `flux-panel.db-wal`

备份时请一起考虑。

---

## 说明

如果你只是要部署项目，请优先阅读仓库根目录 [README.md](/c:/Users/Lee/Desktop/dlam/README.md)。

本文件主要用于说明本目录本身的用途，而不是完整部署文档。
