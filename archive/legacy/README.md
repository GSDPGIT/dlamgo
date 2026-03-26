# 归档目录说明

本目录用于存放当前仓库中的历史实现、旧部署文件、移动端工程以及不再作为默认入口的旧脚本。

这些内容被保留的主要原因包括：

- 方便历史版本参考
- 方便迁移时对照旧实现
- 保留原客户端工程
- 保留旧部署脚本以备少量兼容场景使用

---

## 当前归档内容

- `springboot-backend`
  旧版 Java 后端

- `android-app`
  Android 客户端工程

- `ios-app`
  iOS 客户端工程

- `flux.ipa`
  历史 iOS 产物文件

- `docker-compose-v4.yml`
  历史 Compose 配置

- `docker-compose-v6.yml`
  历史 Compose 配置

- `docker-compose-sqlite.yml`
  旧版 SQLite Compose 文件，当前默认入口已统一为根目录 `docker-compose.yml`

- `install.sh`
  历史节点安装脚本原始版本

- `panel_install.sh`
  历史面板安装脚本

- `gost.sql`
  历史 MySQL 初始化文件

---

## 注意

如果你要部署当前默认版本，请不要优先使用本目录中的文件。

当前推荐使用：

- 根目录 `README.md`
- 根目录 `docker-compose.yml`
- 根目录 `.env.example`
- `go-panel-backend`
- `vite-frontend`
- `scripts/install-node.sh`
