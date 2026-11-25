# LinChat 部署记录

## 当前工程

- 工程目录：`E:\LinChat`
- Windows 客户端：Electron + Vue
- 服务端：Go + SQLite；Windows / Linux x64
- 云端目标：腾讯云 `43.138.138.136`，`chat.linblog.site`
- Radmin 本机网卡：`26.232.226.141`

## 1.2.0 娱乐模式

- 已发布：[LinChat 1.2.0](https://github.com/Lin-6499/ChatForPlay/releases/tag/v1.2.0)，发行源码提交 `9d64f18`。Windows 安装包 `LinChat-Setup-1.2.0.exe`，107,112,424 字节，SHA-256：`bd5581e213cc76af4ed94c753d946274e94eba632faa0c697e923b99d2a3f547`。
- 安装版、便携版与 Windows/Linux 服务端已构建，内置服务端与最终测试二进制摘要一致。打包后的 EXE 已连接云端 HTTPS 并验证正式版更新接口。结果：`dist/verification/packaged-check-results.json`。
- 7 个发行附件已上传；GitHub 返回的大小及 SHA-256 与本地一致。结果：`dist/github-release.json`。
- 本机从 1.1.0 升级至 **1.2.0** 并已重新打开，安装位置 `C:\Users\Administrator\AppData\Local\Programs\linchat-desktop\LinChat.exe`。EXE、应用包、内置服务端均与最终构建一致；桌面和开始菜单快捷方式均正确。结果：`dist/verification/installer-results.json`。
- 实际安装版通过独立验证配置连接云端 HTTPS 登录页，并成功请求正式 GitHub 更新接口。结果：`dist/verification/installed-check-results.json`。
- 已使用 Windows PowerShell 5.1 从公开发行地址完整下载安装包，大小及 SHA-256 与固定清单一致；正式最新版响应会向 1.1.0 客户端提示更新，1.2.0 不会收到重复升级提示。结果：`dist/verification/download-results.json`、`published-update-check.json`。
- 战役房间、固定规则版本、角色卡与资源、服务器掷骰、公开／私密日志、结束摘要、故事提案和暂停／继续已实现。
- 支持手动主持、Grok／OpenAI 兼容内置主持、战役令牌与外部自动主持；HTTP 接口、Node.js 22+ 机器人示例和 stdio MCP 接入见 [娱乐模式文档](docs/ENTERTAINMENT.md)。
- Go 集成测试及 `go vet`、7 项 JS 逻辑测试、13 项娱乐模式端到端场景、11 项原有 Electron 场景通过。内置与外部模型流程使用本机工具调用替身验证。
- 桌面 1264×811、800×654 与移动 390×780 完成截图复核，768px／320px 完成主操作命中与溢出检查。草稿保留和移动错误重试经修正后的复核结论为 `ship`，范围限于该修复批次。
- 2026-11-25 云端从 1.1.0 升级至 **1.2.0**；备份目录：`/var/backups/linchat/update-20261125-103617-tY9Itn/`，包含完整数据及旧程序。
- SQLite 迁移至 `user_version=3`，完整性检查为 `ok`。逐项比较保留原有 3 个用户、4 个会话、9 条成员关系、36 条消息、2 条提及关系；附件记录为 0。结果：`dist/verification/cloud-upgrade-after.json`。
- 云端 Linux 二进制 SHA-256：`3b19fe5b998c0a701519ef0fce4bbfc55768a4dbb92b1b98d27318c8594851ff`。
- 6 项公网检查通过，JS/CSS 与已验证构建逐字节一致；战役、主持秘密、模型设置未登录均返回 401。结果：`dist/verification/cloud-check-results.json`。
- 云端模型尚未配置。管理员需填写自己的 API 地址、有效模型 ID 与密钥；云端和本机主机各自保存配置。数据目录新增 `ai-secret.key`，权限为 `0600`，备份必须包含该文件。
- 本次临时 SSH 公钥、服务器暂存目录、本地私钥已移除，原有 SSH 授权保留。结果：`dist/verification/cloud-upgrade-cleanup.json`。

## 1.1.0 功能更新（历史记录）

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
- 2026-11-25 已通过 SSH 将云端从 1.0.0 升级到 1.1.0。用户在本机 SSH 窗口完成密码登录后，使用仅限本次升级的临时密钥执行备份、部署和验证。
- 完整数据及旧程序备份：`/var/backups/linchat/update-20261125-093806-5yxvs1/`。
- 迁移后 SQLite `user_version=2`、完整性检查为 `ok`；备份与在线数据库逐项比较，原有 3 个用户、4 个会话、9 条成员关系、33 条消息均保留，附件记录仍为 0。结果：`dist/verification/cloud-upgrade-after.json`。
- 公网 HTTPS 健康接口已确认 1.1.0，5 项公网检查通过；线上 JS/CSS 与已验证的 1.1.0 构建产物逐字节一致。结果：`dist/verification/cloud-assets-1.1.0.json`。
- 已移除本次临时 SSH 公钥、服务器暂存文件、本地私钥及诊断日志，其余 SSH 授权保留。结果：`dist/verification/cloud-upgrade-cleanup.json`。
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
- 本机及公网健康接口已返回 `{"ok":true,"version":"1.2.0"}`。
- DNS：`chat.linblog.site A 43.138.138.136`。
- HTTPS：Let's Encrypt 证书，有效期至 2027-02-22。
- 自动续期：现有 `certbot.timer`，增加仅针对 chat 域名的续期后 Nginx reload hook。
- 初始化：管理员已创建；新成员通过管理员生成的邀请链接注册，已有账号直接登录。
- Linux 二进制 SHA-256：`3b19fe5b998c0a701519ef0fce4bbfc55768a4dbb92b1b98d27318c8594851ff`。

## 验证状态

- Go 集成测试覆盖：私聊隔离、WebSocket 实时消息、编号去重、邀请次数、跨来源请求拒绝、改密会话撤销、文件权限和大小限制、历史分页与数据库重启持久化。
- 前端 TypeScript 检查及 Vite 构建已通过。
- Go 静态检查已通过。
- Electron 11 项冒烟验证已通过，包括三账号聊天、附件、建群、多人提及与提醒、更新提示下发送、IPC 权限边界和 Radmin 实际网卡切换。
- 本机存在 HTTP 系统代理。桌面程序针对 Radmin / 私网地址使用直连；云端优先采用系统代理，遇到代理或传输故障时尝试直连，始终验证 TLS 证书。
- Electron 安装包输出：`dist/desktop/<版本>/`；1.0.1 起采用当前用户一键安装、安装后自动启动，并提供源码目录下的自动下载安装入口。
- 公网 6 项检查通过：HTTPS 健康接口、Vue 静态资源与响应头、未登录隔离、跨来源写入拒绝、战役／秘密／模型配置鉴权、初始化令牌保护。结果：`dist/verification/cloud-check-results.json`。
- 实际打包的 Windows EXE 已启动并成功连接云端 HTTPS 引导页，版本号和数据目录正确。结果：`dist/verification/packaged-check-results.json`。
- 桌面版 1.2.0 已完成当前用户静默升级，安装后 EXE、应用包和内置服务端与构建产物一致；桌面及开始菜单快捷方式均指向已安装程序。结果：`dist/verification/installer-results.json`。
- 已安装的 1.2.0 程序已连接云端 HTTPS 并显示登录页，使用独立验证配置目录，并验证真实 GitHub 更新检查。结果：`dist/verification/installed-check-results.json`。
- 原站点回归：`https://ht.linblog.site/` 返回 200；`https://files.linblog.site/` 根路径部署前后均返回 404。
- 交付后已从 `ubuntu` 的 `authorized_keys` 移除本次 `linchat-120-deploy-session` 临时公钥；其余 SSH 授权保留。

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
