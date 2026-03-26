# DLAMGO

## 项目简介

DLAMGO 是一个以 **Go + SQLite + Vite** 为默认运行方案的流量转发管理面板。

这个仓库保留了原项目中的多套代码与客户端资源，但当前默认、推荐、实际可部署的面板方案是：

- 前端：`vite-frontend`
- 后端：`go-panel-backend`
- 数据库：`SQLite`
- 节点通信：基于仓库内的 `go-gost`
- 推荐部署方式：`Docker Compose`

当前仓库仍保留以下目录，主要用于兼容、迁移或参考：

- `archive/legacy/springboot-backend`：旧版 Java 后端，**不再作为默认部署方案**
- `archive/legacy/android-app`：Android 客户端工程
- `archive/legacy/ios-app`：iOS 客户端工程
- `archive/legacy/docker-compose-v4.yml` / `archive/legacy/docker-compose-v6.yml`：历史 Compose 文件，**归档/兼容用途**
- `archive/legacy/panel_install.sh` / `archive/legacy/install.sh`：历史脚本，**归档/兼容用途**

如果你只是想把面板跑起来，请优先使用本文档中的 **Docker Compose 部署**。

---

## 主要功能

- 支持 TCP / UDP 转发
- 支持端口转发与隧道转发两种模式
- 支持按用户维度控制流量、转发数量、到期时间
- 支持按用户-隧道维度进行授权与配额控制
- 支持转发限速规则
- 支持节点在线状态、基础系统信息展示
- 支持通过 WebSocket 向节点下发服务、链路、限速等控制指令
- 默认使用 SQLite，部署轻量，适合低配置服务器

---

## 当前默认架构

### 面板架构

- 浏览器访问 `vite-frontend`
- `vite-frontend` 通过 Nginx 反向代理请求 `go-panel-backend`
- `go-panel-backend` 使用 SQLite 保存业务数据
- 节点端通过 `go-gost` 与后端建立连接

### 默认端口

- 前端默认端口：`6366`
- 后端默认端口：`6365`

---

## 目录说明

### 与部署直接相关

- `docker-compose.yml`：默认 Docker 编排文件
- `.env.example`：带中文注释的环境变量模板
- `go-panel-backend`：Go 后端
- `vite-frontend`：前端
- `go-gost`：节点端通信与控制逻辑
- `scripts/install-node.sh`：当前默认节点安装脚本

### 历史/兼容目录

- `archive/legacy/springboot-backend`：旧 Java 后端，已归档
- `archive/legacy/android-app`：Android 客户端，非默认部署路径
- `archive/legacy/ios-app`：iOS 客户端，非默认部署路径
- `archive/legacy/flux.ipa`：历史产物，非默认部署路径
- `archive/legacy/docker-compose-v4.yml`：历史 Compose 配置
- `archive/legacy/docker-compose-v6.yml`：历史 Compose 配置
- `archive/legacy/docker-compose-sqlite.yml`：旧版 SQLite Compose 配置

### 新用户建议优先关注

如果你是第一次接触本项目，建议优先关注这些内容：

- `README.md`
- `DOCKER.md`
- `.env.example`
- `docker-compose.yml`
- `go-panel-backend`
- `vite-frontend`

### 新用户通常可以先忽略

如果你的目标只是部署当前默认版本，以下内容通常可以先忽略：

- `springboot-backend`
- `android-app`
- `ios-app`
- `docker-compose-v4.yml`
- `docker-compose-v6.yml`
- `panel_install.sh`
- `install.sh`
- `flux.ipa`

上面这些目录和文件目前都已经迁移到：

- `archive/legacy/`

### 补充文档

- `DOCKER.md`：Docker 快速部署说明
- `doc/仓库结构说明.md`：仓库结构与归档目录说明
- `doc/发布流程建议.md`：推荐发布流程说明
- `archive/legacy/README.md`：归档目录说明
- `SECURITY.md`：安全说明
- `CONTRIBUTING.md`：贡献指南
- `scripts/README.md`：当前脚本目录说明

### 开源协作文件

仓库当前还包含以下协作与发布辅助文件：

- `.github/ISSUE_TEMPLATE/`：Issue 模板
- `.github/pull_request_template.md`：PR 模板
- `.github/release.yml`：GitHub 自动 Release Notes 分类配置

---

## 部署方式总览

本仓库提供两种主要使用方式：

1. **Docker Compose 部署**
适合大多数用户，最省事，也是默认推荐方式。

2. **源码运行**
适合开发、调试或二次开发。

---

## 一、Docker Compose 部署

## 1. 环境要求

请先确保你的机器已经安装：

- Docker Desktop，或 Docker Engine + Docker Compose

建议最低资源：

- CPU：1 核
- 内存：1 GB
- 磁盘：2 GB 以上可用空间

---

## 2. 准备环境变量

在仓库根目录执行：

```bash
cp .env.example .env
```

然后编辑 `.env`，至少建议修改下面几项：

```env
# 后端映射端口
BACKEND_PORT=6365

# 前端映射端口
FRONTEND_PORT=6366

# JWT 密钥，强烈建议修改
JWT_SECRET=请改成你自己的长随机字符串

# 管理员用户名
ADMIN_USERNAME=admin_user

# 管理员密码，强烈建议修改
ADMIN_PASSWORD=请改成你自己的强密码

# 跨域来源，留空时更适合默认反代场景
CORS_ALLOWED_ORIGINS=

# 每分钟允许的登录次数
LOGIN_RATE_LIMIT_PER_MINUTE=12
```

### 字段说明

- `BACKEND_PORT`
  映射到宿主机的后端端口，默认 `6365`

- `FRONTEND_PORT`
  映射到宿主机的前端端口，默认 `6366`

- `JWT_SECRET`
  登录令牌签名密钥，**强烈建议手动设置**

- `ADMIN_USERNAME`
  初始管理员用户名，默认 `admin_user`

- `ADMIN_PASSWORD`
  初始管理员密码，**强烈建议手动设置**

- `CORS_ALLOWED_ORIGINS`
  允许跨域来源，留空时更适合本地和默认反代场景

- `LOGIN_RATE_LIMIT_PER_MINUTE`
  登录接口限速，默认每分钟 `12` 次

---

## 3. 启动服务

在仓库根目录执行：

```bash
docker compose up --build -d
```

启动完成后默认访问地址：

- 前端：`http://127.0.0.1:6366`
- 后端：`http://127.0.0.1:6365`

---

## 4. 查看运行状态

查看容器状态：

```bash
docker compose ps
```

查看后端日志：

```bash
docker compose logs -f backend
```

查看前端日志：

```bash
docker compose logs -f frontend
```

---

## 5. 首次登录

如果你在 `.env` 中显式设置了：

```env
ADMIN_USERNAME=admin_user
ADMIN_PASSWORD=你的密码
```

那么直接使用这组账号密码登录即可。

如果你没有设置 `ADMIN_PASSWORD`，后端会在首次启动时自动生成一个随机密码，并写到日志中。你需要通过下面命令查看：

```bash
docker compose logs backend
```

建议首次登录后立刻修改密码。

---

## 6. 数据保存位置

Docker 部署默认使用命名卷：

- `sqlite_data`

SQLite 数据库文件保存在容器内：

- `/data/flux-panel.db`

如果你删除容器但不删除卷，数据仍会保留。

---

## 7. 停止、重启、删除

停止服务：

```bash
docker compose stop
```

重启服务：

```bash
docker compose restart
```

删除容器但保留数据卷：

```bash
docker compose down
```

删除容器并删除数据卷：

```bash
docker compose down -v
```

---

## 8. 升级

如果你更新了代码，升级流程如下：

```bash
git pull
docker compose build --no-cache
docker compose up -d
```

如果你不想强制重新构建，也可以先试：

```bash
git pull
docker compose up -d --build
```

---

## 二、源码运行（开发/调试）

## 1. 后端运行

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
APP_ADDR=127.0.0.1:6365 \
DATABASE_PATH=./data/flux-panel.db \
JWT_SECRET=your-secret \
ADMIN_USERNAME=admin_user \
ADMIN_PASSWORD=your-password \
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

## 2. 前端运行

进入目录：

```bash
cd vite-frontend
```

安装依赖：

```bash
npm install --legacy-peer-deps
```

本地开发时，确认 `vite-frontend/.env.development` 中包含：

```env
VITE_API_BASE=http://127.0.0.1:6365
```

启动开发服务器：

```bash
npm run dev
```

生产构建：

```bash
npm run build
```

---

## 三、节点接入

节点接入依赖面板中的“节点管理”和仓库中的 `go-gost` / `scripts/install-node.sh`。

## 1. 面板启动后先做的配置

请先登录面板，在“网站配置”里设置：

- `面板后端地址`

这个地址非常重要，用于节点回连面板。

建议填写格式：

- `ip:port`
- `http://ip:port`
- `https://域名:端口`

如果你做了 HTTPS / WSS 反代，也可以填写你的反代地址。

---

## 2. 创建节点

在面板里进入“节点管理”：

1. 新建节点
2. 填写入口 IP、服务器 IP、端口范围
3. 保存后获取安装命令

面板会生成一条节点安装命令。

---

## 3. 节点安装

如果你使用面板生成的安装命令，通常会类似：

```bash
curl -fsSL https://raw.githubusercontent.com/你的仓库/你的分支/scripts/install-node.sh -o ./install-node.sh && chmod 700 ./install-node.sh && ./install-node.sh -a 面板地址 -s 节点密钥
```

说明：

- `-a`：面板地址
- `-s`：节点密钥

节点安装完成后，会尝试连接面板并上报状态。

---

## 四、默认安全策略

当前默认实现已经处理了以下问题：

- 管理员密码不再强制使用固定默认值
- 登录密码使用更安全的密码哈希
- 管理端 WebSocket 改为短时 `ticket` 鉴权
- 节点上报 HTTP 请求改为通过 Header 传递节点密钥
- 登录接口支持基础限速
- 面板默认采用 Go + SQLite，降低内存占用

---

## 五、GitHub Actions 自动构建

仓库当前包含一个默认工作流：

- `.github/workflows/docker-build.yml`

当前工作流会自动执行：

1. Go 后端构建检查
2. 前端依赖安装与构建检查
3. Docker 后端镜像构建检查
4. Docker 前端镜像构建检查

这个工作流默认用于 **持续集成校验**，不会默认帮你推送镜像到 Docker Hub。

仓库现在还包含一个独立的发布工作流：

- `.github/workflows/release.yml`

它会在你推送 `v*` 标签时自动：

1. 构建 Go 后端发布二进制
2. 构建前端静态产物
3. 校验 Docker 镜像可构建
4. 自动创建 GitHub Release
5. 上传二进制、前端构建包、`docker-compose.yml`、`.env.example` 等附件

此外，仓库还包含 GitHub 自动 Release Notes 配置文件：

- `.github/release.yml`

这个文件用于控制 GitHub 自动版本说明的分类规则。

如果你只是希望：

- 提交代码后自动检查有没有构建问题

那么当前工作流已经够用。

如果你以后还希望自动推送 Docker 镜像，仓库现在还包含：

- `.github/workflows/docker-publish.yml`

这个工作流会在 `v*` 标签触发时尝试推送镜像，并默认发布：

- `linux/amd64`
- `linux/arm64`

同时自动生成：

- `latest`
- 当前标签名

两个镜像标签。

前提是你已经配置仓库 Secrets：

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

建议继续保持“CI 校验”“Release 附件发布”“Docker 镜像推送”三者分离，不要把当前 `release.yml` 和镜像推送强绑在一起。

---

## 六、已知说明

## 1. Java 后端仍在仓库中

仓库里仍保留 `archive/legacy/springboot-backend`，但当前默认部署和本文档都不再使用它。

如果你只是部署面板，请直接使用：

- `go-panel-backend`
- `vite-frontend`
- `docker-compose.yml`

## 2. 归档说明

以下内容当前属于归档/兼容保留范围：

- `springboot-backend`
- `android-app`
- `ios-app`
- `docker-compose-v4.yml`
- `docker-compose-v6.yml`
- `panel_install.sh`
- `install.sh`
- `flux.ipa`

这些文件现在都在：

- `archive/legacy/`

这些内容仍然保留在仓库中，主要用于：

- 历史兼容
- 迁移参考
- 客户端工程保留
- 原始脚本保留

默认部署、默认维护、默认阅读路径都不再依赖它们。

## 3. 前端安装参数

前端依赖树中存在上游 peer dependency 约束，因此在某些环境中需要使用：

```bash
npm install --legacy-peer-deps
```

Docker 构建中已经按这个方式处理。

---

## 七、备份与迁移

## 1. 备份 SQLite 数据库

Docker 部署中，数据库在容器内：

- `/data/flux-panel.db`

你可以通过以下方式备份：

```bash
docker compose exec backend sh -c "cp /data/flux-panel.db /data/flux-panel.db.bak"
```

或者直接导出卷内容。

## 2. 源码运行时的数据库

如果你是源码运行，只需要备份：

- `go-panel-backend/data/flux-panel.db`

以及可能存在的：

- `flux-panel.db-shm`
- `flux-panel.db-wal`

---

## 八、常见问题

## 1. `docker compose build` 拉镜像失败

请检查：

- Docker Desktop 是否已启动
- Docker 是否配置了正确代理
- 是否能正常拉取：

```bash
docker pull alpine:3.20
docker pull golang:1.23-alpine
docker pull node:20.19.0
docker pull nginx:stable-alpine
```

## 2. 登录后无法看到节点在线

请检查：

- 节点是否真的执行了安装脚本
- 面板“网站配置”中的后端地址是否正确
- 节点与面板之间的网络是否可达
- 反代是否放通了 `/system-info` WebSocket

## 3. 前端能打开但接口报错

请检查：

- 后端容器是否启动成功
- Nginx 是否正确代理 `/api/v1`
- 浏览器访问：

```bash
http://你的地址/api/v1/captcha/check
```

## 4. 端口被占用

修改 `.env` 中：

```env
BACKEND_PORT=新的端口
FRONTEND_PORT=新的端口
```

然后重启：

```bash
docker compose up -d --build
```

---

## 九、发布版本建议

推荐阅读：

- `doc/发布流程建议.md`
- `CHANGELOG.md`

简要建议如下：

1. 本地先完成后端、前端、Docker 联调
2. 更新文档
3. 推送到 `main`
4. 等 GitHub Actions 通过
5. 再打 tag 与发版

推荐版本号格式：

- `v1.0.0`
- `v1.0.1`
- `v1.1.0`

推荐标签规范：

- `v主版本.次版本.修订版本`

例如：

- `v0.1.0`
- `v0.1.1`
- `v0.2.0`

不建议使用：

- `release-1`
- `test`
- `final`

---

## 十、推荐使用方式

如果你只是想稳定使用，请按下面顺序：

1. 修改 `.env`
2. 执行 `docker compose up --build -d`
3. 登录面板
4. 在网站配置里设置“面板后端地址”
5. 创建节点并获取安装命令
6. 节点安装完成后再创建隧道、用户和转发

---

## 十一、免责声明

本项目仅供学习、研究与合法合规用途使用。

请勿将本项目用于任何违法、滥用、攻击、绕过授权或其他不当用途。

使用本项目造成的风险，包括但不限于：

- 服务异常
- 数据丢失
- 节点失联
- 网络封禁
- 法律风险

均由使用者自行承担。

---

## 十二、许可证

本项目遵循仓库中的 `LICENSE`。
