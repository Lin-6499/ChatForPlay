# LinChat 部署记录

## 当前工程

- 工程目录：`E:\LinChat`
- Windows 客户端：Electron + Vue
- 服务端：Go + SQLite；Windows / Linux x64
- 云端目标：腾讯云 `43.138.138.136`，`chat.linblog.site`
- Radmin 本机网卡：`26.232.226.141`

## 1.1.0 功能更新

- 已发布：[LinChat 1.1.0](https://github.com/Lin-6499/ChatForPlay/releases/tag/v1.1.0)，发行源码提交 `6569168`。
- Windows 安装包：`LinChat-Setup-1.1.0.exe`，106,898,765 字节；SHA-256：`759c7717077de5e39b1cbf323bf92a71776c7f28a56e91dd261bb2753ef08ff3`。
- 安装版、便携版、Windows/Linux 服务端、云端升级脚本及校验清单已上传，GitHub 返回的摘要均与本地一致。
- 本机已从 1.0.1 升级至 1.1.0，安装后的 EXE、应用包及内置服务端与构建产物一致；已修正一处指向旧开发目录的开始菜单快捷方式，桌面及开始菜单均指向已安装程序。
- 实际安装版已验证连接云端 HTTPS 登录页，并成功访问 GitHub 正式版更新接口。结果：`dist/verification/installed-check-results.json`。
- 已从公开发行地址完整下载 1.1.0 安装包并通过大小及 SHA-256 校验；真实 GitHub 最新版响应通过更新识别验证，当前 1.1.0 不会被提示重复升级。结果：`dist/verification/published-update-check.json`。
- 群聊及公共大厅支持 @成员，持久化目标账号及文字位置；旧数据库自动增加 `message_mentions` 表和索引。
- 桌面版增加 GitHub 正式版本检查：启动后自动检查、每 6 小时检查，以及连接中心、托盘和个人设置的手动入口。
- Go 集成测试和静态检查通过；6 项提及/更新逻辑测试通过；11 项 Electron 场景通过，含真实多人提及、未读标记、后台通知、窄屏及更新提醒。
- 更新提示在页面顶部独立占位；1264×811 和 390×780 下，提示打开时均通过真实鼠标事件发送验证。复核确认发送按钮遮挡问题已解决。
- 云端目前仍为 1.0.0。此次尝试 SSH 登录被拒绝，腾讯云控制台需要重新登录；1.1.0 云端升级等待可用的服务器授权。
- 云端升级脚本：`deploy/update-cloud.sh`。验证新二进制 SHA-256 后停止服务、备份完整数据目录及旧程序，再更新并检查指定版本健康状态。失败恢复旧程序，不回滚已接受的新消息。

```bash
sudo bash update-cloud.sh ./linchat-linux-amd64 <SHA256SUMS.txt 中的 Linux 二进制摘要> 1.1.0
```

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

## 桌面一键安装发行（1.0.1 历史记录）

- 公开版本：[LinChat 1.0.1](https://github.com/Lin-6499/ChatForPlay/releases/tag/v1.0.1)。
- 安装包：`LinChat-Setup-1.0.1.exe`，106,885,608 字节；GitHub 返回的 SHA-256 与仓库内 `desktop-release.json` 一致。
- 安装包 SHA-256：`871342839d6c84e85f4965ba5ad4da5c1dbb803eda1a286b3f54d11626189376`。
- 普通用户双击安装包即可自动安装、生成快捷方式并启动；源码 ZIP 完整解压后双击 `一键安装使用.cmd` 也可自动下载安装。
- 已从不含依赖及构建文件的源码 ZIP 验证 CMD 入口，包含中文与空格的解压路径可正常使用，检测到当前已安装版本后直接启动。结果：`dist/verification/source-install-results.json`。
- 已在 Windows PowerShell 5.1 中使用源码 ZIP 内的安装脚本，从 GitHub 公开地址完整下载实际安装包，并通过文件大小及 SHA-256 校验。结果：`dist/verification/download-results.json`。

## 常用运维命令

```bash
sudo systemctl status linchat
sudo journalctl -u linchat -n 50 --no-pager
sudo systemctl restart linchat
sudo nginx -t
```

备份请先停止 LinChat，再复制完整 `/var/lib/linchat`，完成后启动。现有 Nginx 配置快照位于 `/var/backups/linchat/`。
