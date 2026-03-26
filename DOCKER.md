# Docker 部署说明

本文件是仓库根目录 [README.md](/c:/Users/Lee/Desktop/dlam/README.md) 的 Docker 快速补充版。

如果你只想尽快把项目跑起来，可以直接看这里。

---

## 一、前提条件

请先确保本机已经安装：

- Docker Desktop

或：

- Docker Engine
- Docker Compose

---

## 二、准备配置文件

在仓库根目录执行：

```bash
cp .env.example .env
```

然后至少修改下面几个变量：

```env
BACKEND_PORT=6365
FRONTEND_PORT=6366
JWT_SECRET=请改成你自己的长随机字符串
ADMIN_USERNAME=admin_user
ADMIN_PASSWORD=请改成你自己的强密码
```

---

## 三、启动

在仓库根目录执行：

```bash
docker compose up --build -d
```

默认访问地址：

- 前端：`http://127.0.0.1:6366`
- 后端：`http://127.0.0.1:6365`

---

## 四、常用命令

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

停止服务：

```bash
docker compose stop
```

删除容器但保留数据：

```bash
docker compose down
```

删除容器和数据卷：

```bash
docker compose down -v
```

---

## 五、数据保存位置

Docker 部署默认使用命名卷：

- `sqlite_data`

数据库文件位于容器内：

- `/data/flux-panel.db`

如果你执行的是：

```bash
docker compose down
```

那么数据库仍会保留。

只有执行：

```bash
docker compose down -v
```

才会把卷一起删除。

---

## 六、说明

- 当前默认部署方案是：`Go 后端 + SQLite + Vite 前端`
- 前端通过 Nginx 代理 `/api/v1`、`/flow/*`、`/system-info`
- 如果你需要完整安装文档、节点接入说明、升级和备份方法，请阅读根目录 [README.md](/c:/Users/Lee/Desktop/dlam/README.md)
