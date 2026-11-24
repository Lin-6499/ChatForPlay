# LinChat 部署记录

## 当前工程

- 工程目录：`E:\LinChat`
- Windows 客户端：Electron + Vue
- 服务端：Go + SQLite；Windows / Linux x64
- 云端目标：腾讯云 `43.138.138.136`，`chat.linblog.site`
- Radmin 本机网卡：`26.232.226.141`

## 云端状态

**已上线：https://chat.linblog.site**

- 使用已有 Nginx 增加独立虚拟主机。
- 聊天进程以独立 `linchat` 系统用户、systemd 服务运行，已设置开机启动。
- 程序：`/opt/linchat/bin/linchat`
- 数据：`/var/lib/linchat`
- 监听：`127.0.0.1:8787`
- systemd：`/etc/systemd/system/linchat.service`
- Nginx：`/etc/nginx/sites-available/linchat`，链接到 `sites-enabled/linchat`
- 本机健康接口已返回 `{"ok":true,"version":"1.0.0"}`。
- DNS：`chat.linblog.site A 43.138.138.136`。
- HTTPS：Let's Encrypt 证书，有效期至 2027-02-22。
- 自动续期：现有 `certbot.timer`，增加仅针对 chat 域名的续期后 Nginx reload hook。
- 初始化：管理员已创建；新成员通过管理员生成的邀请链接注册，已有账号直接登录。
- Linux 二进制 SHA-256：`ed9f92f339bb308996cf1ea7253bab252fe9a5a828324752ab82a152135cbf92`。

## 验证状态

- Go 集成测试覆盖：私聊隔离、WebSocket 实时消息、编号去重、邀请次数、跨来源请求拒绝、改密会话撤销、文件权限和大小限制、历史分页与数据库重启持久化。
- 前端 TypeScript 检查及 Vite 构建已通过。
- Go 静态检查已通过。
- Electron 7 项冒烟验证已通过，包括三账号聊天、附件、建群、IPC 权限边界和 Radmin 实际网卡切换。
- 本机存在 HTTP 系统代理。桌面程序针对 Radmin / 私网地址使用直连；云端优先采用系统代理，遇到代理或传输故障时尝试直连，始终验证 TLS 证书。
- Electron 安装包输出：`dist/desktop/<版本>/`；1.0.1 起采用当前用户一键安装、安装后自动启动，并提供源码目录下的自动下载安装入口。
- 公网 5 项检查通过：HTTPS 健康接口、Vue 静态资源与响应头、未登录隔离、跨来源写入拒绝、初始化令牌保护。结果：`dist/verification/cloud-check-results.json`。
- 实际打包的 Windows EXE 已启动并成功连接云端 HTTPS 引导页，版本号和数据目录正确。结果：`dist/verification/packaged-check-results.json`。
- 桌面版 1.0.1 已完成当前用户静默安装，安装后 EXE 与构建产物 SHA-256 一致；桌面及开始菜单快捷方式均指向已安装程序。结果：`dist/verification/installer-results.json`。
- 已安装的 1.0.1 程序已连接云端 HTTPS 并显示登录页，使用独立验证配置目录。结果：`dist/verification/installed-check-results.json`。
- 原站点回归：`https://ht.linblog.site/` 返回 200；`https://files.linblog.site/` 根路径部署前后均返回 404。
- 交付后已从 `ubuntu` 的 `authorized_keys` 移除本次 `linchat-deploy-session` 临时公钥；其余 SSH 授权保留。

## 常用运维命令

```bash
sudo systemctl status linchat
sudo journalctl -u linchat -n 50 --no-pager
sudo systemctl restart linchat
sudo nginx -t
```

备份请先停止 LinChat，再复制完整 `/var/lib/linchat`，完成后启动。现有 Nginx 配置快照位于 `/var/backups/linchat/`。
