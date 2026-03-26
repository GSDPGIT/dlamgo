# 安全说明

## 支持范围

当前默认支持和维护的部署方案是：

- `go-panel-backend`
- `vite-frontend`
- `docker-compose.yml`

以下内容属于归档或历史保留，不作为当前默认安全维护范围：

- `archive/legacy/springboot-backend`
- `archive/legacy/android-app`
- `archive/legacy/ios-app`
- `archive/legacy/install.sh`
- `archive/legacy/panel_install.sh`

## 报告安全问题

如果你发现安全问题，请不要直接公开提交利用细节。

建议做法：

1. 先准备最小复现说明
2. 描述影响范围
3. 描述利用条件
4. 描述临时缓解方式

如果仓库后续启用专门的安全邮箱或私密报告通道，建议优先使用私密方式报告。

在此之前，如需公开提交 Issue，请避免直接公开：

- 有效密钥
- 管理员口令
- 节点密钥
- 可直接利用的攻击脚本
- 可直接执行的恶意载荷

## 当前安全建议

- 请手动设置强密码和 `JWT_SECRET`
- 建议通过 HTTPS / WSS 暴露面板
- 不要把后端端口直接暴露在公网
- 定期备份 SQLite 数据库
- 使用最新镜像或最新版本标签
