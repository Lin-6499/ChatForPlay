# LinChat

**Electron + Vue 3 + Go 的多人实时通讯软件。** 同一个桌面客户端同时支持：

- **云服务器模式**：连接 `https://chat.linblog.site`；也可以在电脑、手机浏览器使用。
- **Radmin 虚拟局域网模式**：在一台 Windows 电脑启动内置主机，伙伴加入同一 Radmin 网络后，连接该主机的虚拟 IP。

项目目录：`E:\LinChat`。

## 功能

- 管理员首次初始化、邀请注册、账号登录与个人密码修改。
- 公共大厅、一对一私聊、自建群聊、群主添加成员。
- WebSocket 实时推送、多人在线状态、会话未读数、已读人数。
- 群聊和公共大厅 @成员，支持昵称/账号搜索、多人提及、高亮及“@我”未读提醒。
- 文字、图片、文件，支持图片粘贴、拖放文件、上传进度和失败重试。
- 服务端落盘确认、消息编号去重、历史分页、重连后增量同步。
- Electron 连接中心、保存服务端地址、托盘常驻、系统通知。
- Electron 内置 Go 服务端，可一键启动 Radmin 主机或仅本机体验。
- 桌面新版后台下载与校验，点击“重启并更新”即可静默升级；支持连接中心、托盘和个人设置入口。
- 娱乐模式：文字战役、角色卡、服务器掷骰、场景与摘要存档、私密资料和房主确认的设定提案。
- 支持 Grok / OpenAI 兼容 API 主持、战役专用令牌、独立机器人与 stdio MCP 连接器。

当前默认限制：每条文字 4,000 字符、每个附件 25 MiB、累计附件 5 GiB、自建群最多 100 人。
这是初版工程配置上限，未将其作为实测并发容量保证。语音/视频通话不在本版功能内。

## 快速使用（Windows）

### 安装并选择位置

**[下载 LinChat 1.2.2 安装包](https://github.com/Lin-6499/ChatForPlay/releases/download/v1.2.2/LinChat-Setup-1.2.2.exe)**

1. 下载 `LinChat-Setup-1.2.2.exe`。
2. 双击运行，在安装向导中点击**浏览**或直接输入路径，例如 `D:\应用\LinChat`，再点击**安装**。安装后创建桌面、开始菜单快捷方式；最后一页可选择是否立即运行。
3. 点击预置的 **“LinChat 云端”**，用自己的账号登录。新成员先使用管理员发来的邀请链接注册。

适用于 Windows 10/11 64 位，为当前 Windows 用户安装，支持自选有写入权限的磁盘和文件夹，普通使用无需安装 Node.js、Go 或执行编译命令。

**如果下载的是 GitHub 源码 ZIP：** 先完整解压，再双击根目录的 **`一键安装使用.cmd`**。脚本自动获取对应版本的安装包、验证 SHA-256，再打开可选择路径的安装向导。**`启动LinChat.cmd`** 会直接打开已安装的同版或更新版本，未安装时才显示安装向导。

**修改已有安装位置：** 重新运行同版或新版安装包，或双击 `一键安装使用.cmd`，在向导中选择新路径。向导默认显示原位置，安装后会更新快捷方式。账号、连接设置与本机聊天数据保存在 `%APPDATA%\linchat-desktop`，与程序安装目录分开。

安装脚本优先使用本地完整安装包或已校验的下载缓存，首次在线下载约 103 MB。安装包缓存位于 `%LOCALAPPDATA%\LinChat\downloads\`；这是下载缓存，程序安装位置由向导选择。网络失败时窗口会显示原因和直接下载地址。

[查看发行版本与便携版](https://github.com/Lin-6499/ChatForPlay/releases/latest)

开发者本地构建后的文件：

```text
dist/desktop/1.2.2/LinChat-Setup-1.2.2.exe       可选择安装目录的安装版
dist/desktop/1.2.2/LinChat-Portable-1.2.2.exe    便携版
dist/desktop/1.2.2/latest.yml                  自动更新版本与 SHA-512 清单
dist/server/linchat-windows-amd64.exe           独立 Windows 服务端
dist/server/linchat-linux-amd64                 独立 Linux 服务端
```

### 连接云端

1. 打开 LinChat，选择“加入聊天”。
2. 填写实际完成部署的服务端地址，例如 `https://chat.linblog.site`。
3. 使用管理员提供的邀请创建账号，或直接登录。

默认地址是预置连接资料；只有云端部署、DNS 和 HTTPS 均完成后才能使用。实际部署状态见 `DEPLOYMENT.md`。

### @ 群成员

在群聊或公共大厅输入 `@`，也可以点击输入框下方的 **@ 图标**：

1. 输入昵称或账号筛选当前会话的成员。
2. 点击候选成员，或用 ↑ / ↓ 选择、Enter / Tab 确认。
3. 继续输入消息，可再次输入 `@` 添加其他成员，最后发送。

候选列表同时显示昵称和账号，便于区分同名成员。输入区显示将提醒的人数；手动修改已选中的名字会取消这处提及，重新选择即可。Esc 关闭候选列表，Shift + Enter 换行，中文输入法确认文字不会误发消息。

被提及的人在消息中看到 **“提到了你”** 和高亮名字，未读会话显示 **“@我”**。离线期间的提醒会保留到重新连接，阅读消息后清除未读标记。桌面最小化时会收到注明“@了你”的系统通知；网页系统通知需要网站通知权限。

这项功能需要连接到 1.1.0 或更高版本的服务端。本机/Radmin 主机使用新版内置服务端，云端需由服务器管理员同步升级。

### 娱乐模式与主持机器人

点击左侧**骰子图标 → 娱乐模式 → 新建战役**。选择成员、固定规则版本，准备角色卡，再由房主继续战役。

- 玩家通过**提交行动**或在战役群里 **@主持人**推进剧情；普通闲聊不触发模型。
- 手动主持可直接使用；内置 AI 由服务器管理员在**模型设置**中配置自己的 API 地址、模型和密钥。
- 外部智能体使用房主签发的战役专用令牌。仓库提供可运行的自动主持示例和 stdio MCP 连接器。
- 秘密资料由服务端单独授权；内置公开主持只读取公开上下文。骰点由服务器产生并记录，失败重试保留已有骰点。
- 结束本次时保存摘要，下次继续已有角色、资源、场景和日志。

需要连接 **1.2.0 或以上的服务端**。详见 [娱乐模式、API 与 MCP 接入文档](docs/ENTERTAINMENT.md)。

### 桌面自动更新

1.2.2 起，Windows 安装版启动约 10 秒后自动检查 GitHub 正式发行版，运行期间每 6 小时再次检查。发现新版后在后台下载并验证 SHA-512，显示下载进度。完成后点击 **“重启并更新”**，即可自动关闭应用、静默安装并重新打开，不再显示安装向导。下载期间可继续聊天；不会在聊天中突然强制重启。

手动入口：连接中心底部 **“检查更新”**、托盘菜单 **“检查更新”**、聊天页面 **“个人设置 → 检查更新”**。网络失败时可以稍后重试。检查只接受本仓库已上传的正式版 Windows 安装包，不提示降级或预览版本。

**1.2.1 及更早版本需要先安装一次 1.2.2，之后才具备应用内自动更新能力。** 更新沿用原安装目录并保留用户数据；内置主机先有序停止，更新后恢复主机和原连接。便携版及源码开发环境提供下载安装版入口，不会覆盖自身。

较旧的云端页面仍可通过原“下载更新”按钮调用新客户端的应用内流程，也可从桌面托盘进入“检查更新”查看准确进度。自动更新缓存位于 `%LOCALAPPDATA%\linchat-desktop-updater\`。

### 在 Radmin 网络创建空间

1. 主机和其他 Windows 成员安装并启动 [Radmin VPN](https://www.radmin-vpn.com/)。
2. 主机创建 Radmin 网络，其他成员加入相同网络。
3. 主机打开 LinChat → “本机创建空间” → “启动 Radmin 聊天主机”。
4. 点击“初始化聊天空间”，设置管理员账号。
5. 将连接中心显示的地址发给成员，例如 `http://26.x.x.x:8787`。
6. 管理员在聊天界面点击“邀请成员”，生成邀请链接。成员可以直接用浏览器打开该链接，或者将 `#invite=` 后的邀请码粘贴到 Electron 的注册表单。
7. 如首次连接被 Windows 防火墙阻止，在主机电脑上以管理员身份运行：

```powershell
powershell -ExecutionPolicy Bypass -File "E:\LinChat\scripts\allow-radmin-firewall.ps1"
```

该脚本只建立 LinChat 自己的 TCP 8787 入站规则，限定到 Radmin 网卡和它的当前 IPv4 地址。

**主机必须保持开机、联网且 LinChat 正在运行。** 关闭窗口会进入托盘；托盘菜单选择“退出”会同时停止内置服务端。
主机在线时可以为离线成员保存消息。主机关机时，其他人无法继续向它提交消息；客户端保存本机待发送文字，恢复连接后重试。

Radmin 官网目前列出的客户端平台是 Windows。手机、macOS 和 Linux 成员可使用云端网页入口。实际跨省链路可能使用直连，也可能由 Radmin 中继，传输效果需要双方实测。

### 两种方式的兼容范围

软件、通信协议和数据格式相同；云端与本机**独立部署时是两个独立空间**，账号、群和聊天记录不会自动跨实例同步。

- 仅切换客户端访问入口且连到同一个服务端：使用同一份数据。
- 在同一台电脑将本机体验切换成 Radmin 主机：复用同一个本地数据目录。
- 从 Radmin 主机迁移到云端：停止源服务，备份并搬迁完整数据目录，在目标处设置正确权限后启动。
- 不要让两个服务端同时写同一份 SQLite 数据，不要把正在使用的数据库目录放在网络共享盘。

## 数据与备份

安装/便携版 Electron 默认数据位置：

```text
%APPDATA%\linchat-desktop\connections.json  保存的服务端地址
%APPDATA%\linchat-desktop\server-data\      内置主机数据
```

开发模式数据在 `E:\LinChat\.desktop-data\`。连接中心提供“打开本机数据目录”入口，以该入口显示的路径为准。
Electron 的浏览器登录会话保存在自身用户数据目录中。

服务端目录：

```text
linchat.db          SQLite 数据库（账号、登录会话、邀请、会话、消息、文件元数据）
linchat.db-wal      运行期间可能存在的 SQLite WAL
linchat.db-shm      运行期间可能存在的 SQLite SHM
ai-secret.key      模型凭据加密密钥，备份时必须保留
files/             附件原文件
setup-token        仅首次初始化前存在
首次初始化.txt     仅首次初始化前存在，包含管理员一次性初始化地址
```

最简单可靠的备份方式：停止服务端，复制整个数据目录，再启动服务。恢复时一起还原数据库与 `files/`。
迁移后原登录会话仍可能有效，可在个人设置修改密码来撤销该用户其他会话。
管理员初始密码由使用者自行创建，工程不含默认登录密码。

## 本地开发

要求：Node.js 22.19+、Go 1.26+、Windows x64（本项目桌面打包目标）。

```powershell
cd E:\LinChat
npm ci
npm --prefix web ci
npm run build
npm run dev
```

Web 开发：

```powershell
# 终端一：先构建一次前端，Go embed 需要 web/dist
npm run build:web
go run . -listen 127.0.0.1:8787 -data ./data

# 终端二
npm --prefix web run dev
```

Electron 默认关闭 Node 集成，启用上下文隔离、渲染器沙箱和 Web 安全设置。本机主机管理 IPC 只允许内置连接中心调用。
远程聊天页只允许唤起连接中心和显示有限长度的通知。

若 `npm ci` 在 Electron 官方运行时下载环节超时，可先完成 npm 依赖安装，再运行校验下载脚本：

```powershell
$env:ELECTRON_SKIP_BINARY_DOWNLOAD = '1'
npm ci
Remove-Item Env:ELECTRON_SKIP_BINARY_DOWNLOAD
node scripts/install-electron.cjs
```

脚本下载 Electron 官方发行档案，并使用 npm Electron 包内的 SHA-256 清单校验。

## 构建与验证

```powershell
npm run build
go test -v ./...
go vet ./...
npm run test:desktop
npm run test:features
npm run package
node scripts/packaged-check.cjs
node scripts/cloud-check.cjs
```

`test:desktop` 会创建独立测试数据目录、自动启动 Electron 和内置服务端，使用三个测试账号验证初始化、实时聊天、文件、建群、桌面权限边界；检测到 Radmin 网卡时还会验证切换监听地址后数据保留。结果和截图位于 `dist/verification/`，测试账号不会写入正式数据。

`packaged-check` 启动实际打包的 Windows 程序并连接已部署的 HTTPS 入口，验证安装产物的加载和引导页。`cloud-check` 验证公网 HTTPS、前端资源、未登录访问隔离、来源校验和初始化令牌保护。

`test:features` 验证提及的 Unicode 位置、编辑失效、同名账号区分所依赖的结构，以及更新版本比较、下载地址校验、请求合并和离线状态。Go 集成测试另覆盖提及成员权限、WebSocket 推送、未读与已读、重发去重、SQLite 持久化及旧数据库升级；桌面验证包含真实群聊 @ 操作、中文输入法、窄屏弹层、系统通知和更新提醒。

Radmin 和内网地址在 LinChat 自己的网络会话中使用直连，避免被电脑的 HTTP 系统代理误转发；云端优先使用系统网络设置，遇到代理或连接故障时尝试直连，始终保留 TLS 证书验证。

### 发布桌面安装包

更新根目录 `package.json` 及锁文件中的版本，构建并验证后运行：

```powershell
npm run release:manifest
git add .
git commit -m "Release desktop installer"
git push origin main
npm run release:publish
```

`release:manifest` 生成用于源码一键安装的 `desktop-release.json`，包括对应版本的下载地址、大小和 SHA-256；该文件需要随源码提交。构建产物保存在 `dist/desktop/<版本>/`。

`release:publish` 使用已登录的 Git Credential Manager 凭据，或当前进程中的 `GH_TOKEN` / `GITHUB_TOKEN`，创建 GitHub Release 草稿，上传安装版、便携版、Windows/Linux 独立服务端及校验文件。GitHub 返回的文件摘要全部一致后才公开发布。已公开的版本使用新版本号发布，凭据不会写入项目文件。发行说明可放在 `releases/<版本>.md`。

发布后可验证实际公网下载链路：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-and-start.ps1 -DownloadOnly -ForceDownload
```

## 架构

```text
Electron（连接中心 / 托盘 / 通知 / 本机主机生命周期）
     │
     └─ Vue 页面（网页与桌面共用）
            ├─ HTTP(S)：登录、发送、历史、上传/下载、权限校验
            └─ WS(S)：实时消息、在线状态、已读回执
                       │
                       Go 服务端
                        ├─ SQLite（WAL、唯一消息编号、事务）
                        └─ 本地附件目录
```

消息发送使用带确认的 HTTP，实时接收使用 WebSocket。客户端为每条消息生成随机编号；服务器按发送者与编号去重，数据库提交后返回确认，再推送给会话成员。
客户端重连后按全局递增消息 ID 拉取对应会话的遗漏消息，分页直到补齐。已读游标只前进。

### 主要文件

| 路径 | 用途 |
|---|---|
| `desktop/main.cjs` | Electron 主进程、托盘、IPC、内置服务端 |
| `desktop/preload.cjs` | 隔离的桌面桥接接口 |
| `desktop/index.html`、`launcher.js`、`launcher.css` | 桌面连接中心 |
| `一键安装使用.cmd`、`启动LinChat.cmd` | 普通用户一键安装与启动入口 |
| `scripts/install-and-start.ps1` | 安装检测、下载安装包、SHA-256 校验和启动 |
| `desktop-release.json` | 固定版本安装包的下载地址与校验信息 |
| `web/src/App.vue` | 登录、聊天、通讯录、群组、设置界面 |
| `main.go` | 服务端启动、配置、HTTP 路由、嵌入前端 |
| `auth.go` | 身份验证、会话、邀请、限速 |
| `chat.go` | 会话、权限、消息、已读状态 |
| `hub.go` | WebSocket 连接与广播 |
| `files.go` | 流式上传、文件权限、下载 |
| `store.go` | 数据库结构 |
| `server_test.go` | 跨用户权限、可靠性、附件及持久化测试 |
| `deploy/` | Linux systemd、Docker、Nginx 示例配置 |

## 云端运行

Linux 可直接运行交叉编译二进制，无需在服务器安装 Go、Node 或 Electron：

```bash
./linchat-linux-amd64 \
  -listen 127.0.0.1:8787 \
  -data /var/lib/linchat \
  -url https://chat.linblog.site \
  -name LinChat
```

已有两个网站的服务器应先检查现有反向代理配置，再增加独立域名入口。示例见 `deploy/nginx-chat.conf`；`-url` 使用 HTTPS 时会启用 Secure Cookie，反向代理必须保留 Host 并转发 WebSocket Upgrade。

云端可以选择 `deploy/linchat.service` 的 systemd 运行方式；已有 Docker 环境也可使用 `deploy/compose.yaml`。两者择一，监听端口默认 `8787`，数据库不需要对外开放。

### 传输与隐私边界

云端通过 HTTPS/WSS 保护传输；Radmin 模式依赖 VPN 隧道传输。服务端通过 bcrypt 保存密码哈希，通过 HttpOnly 会话 Cookie 验证身份。修改密码会撤销旧会话并关闭旧实时连接。
消息和附件由服务端存储，**本版没有应用层端到端加密**，服务端管理员可以访问服务器上的内容。
