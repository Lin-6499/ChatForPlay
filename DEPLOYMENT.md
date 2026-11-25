# LinChat 部署记录

## 2026-11-25 实时同步修复（云端 1.2.9）

- 刷新中的新通知排队补刷，连续通知改为节流；重连立即补读临时主持正文，战役与快捷回复每3秒补偿同步。保留主分支 1.2.8 的标识及滚动保护。
- 隔离无头真实 WebSocket 验收6项通过：原地快捷回复、旧响应延迟时补刷、连续正文、断线恢复、正式入档后选项、KeepAlive往返后继续更新。流式、阅读位置、战役位置及Go回归通过。
- 公网健康为1.2.9，线上 JS `index-K6ZA-T3M.js`、CSS `index-CsYM_pc9.css` 与本地构建逐字节一致。
- Linux SHA-256：`8a48243b8a588e7fb4e4b77d8ff2ee312470ccbe0f9d9ea3cc524c655295df76`。完整备份：`/var/backups/linchat/update-20261125-155519-FT4f2i/`。
- 23张表对照保留，84条消息、55条战役事件、4个账号／会话令牌及模型密钥一致。唯一变更为在线用户成员已读位置从83推进至84；另行核对其余成员字段一致。SQLite完整性ok。
- 未向正式战役写测试数据，未打开可见窗口；临时部署目录清理，长期SSH授权保留。证据：`dist/verification/realtime/results.json`、`dist/verification/streaming/cloud.json`。

## 2026-11-25 黑白 Logo 与滚动位置保护（1.2.8）

- 合并远端 1.2.6 窗口几何记忆和 1.2.7 主持流式输出，替换为原创黑白 LC 对话标识。
- 诊断时云端实际健康版本为 1.2.7，线上 JS 包含 `linchat-scroll-v1:`、`data-scroll-key` 和 `reply-prompt`；因此没有将问题直接归结为云端未升级。用户截图缺少新版标签，仍需刷新已加载旧网页。
- 修复异步就绪与恢复帧之间的滚动重置事件可能误写顶部位置；增加专门的事件时序回归。10 项窗口位置检查、11 项聊天阅读位置检查、Go 测试和11项功能测试通过。
- 已通过既有备份升级脚本部署 1.2.8，备份：`/var/backups/linchat/update-20261125-151917-QeGN7A`。升级后本机健康检查通过。
- Linux SHA-256：`4850aadad39520df5c0abf168a27a9d40c552106956c8dd964269aaec2c8853a`。未使用生产账号验收、未写入正式战役测试内容，未启动可见客户端。
- 用户需在现有客户端按一次 Ctrl+R 加载新版云端页面；桌面 EXE 与系统托盘图标需安装新版客户端生效。
- 线上 HTML、JS、CSS、favicon 与本地构建逐字节一致；安装包内 Logo、托盘 PNG 和滚动记忆模块与源文件一致。校验脚本使用 Windows 规范化路径读取 ASAR，避免斜杠差异导致文件缺失误报。验收记录：`dist/verification/scroll-diagnosis/deployed.json`。

## 2026-11-25 主持人流式输出（云端 1.2.7）

- 已部署真正的临时正文预览：内置 AI 的流式工具调用、外部主持示例、MCP `campaign_stream`、成员限定 WebSocket 通知和可恢复快照；最终仍使用 complete 原子入档。
- 公网健康接口已为 `1.2.7`。线上 JS `index-BJj7-61E.js`、CSS `index-B9ExTF9k.css` 与本地构建逐字节一致；匿名访问 stream 返回 401。
- Linux SHA-256：`4a09a20e6c1db1f9da267fecea7b8d4d1ee10a6e9353e081ead81b54b811a92d`。
- 首次部署因健康检查期望旧协议版本而自动恢复旧程序；修正服务端版本为 1.2.7 后重新构建并成功部署。完整备份为 `/var/backups/linchat/update-20261125-150754-OAW59C/` 与 `/var/backups/linchat/update-20261125-150903-vSI3jQ/`。
- 对照第一次更新前备份：19 张旧表全部原记录逐项一致，包含 64 条消息、29 条战役事件、4 个账号、4 个会话令牌。模型加密密钥一致，SQLite 完整性 `ok`；新建临时草稿表。
- Go 集成测试覆盖身份隔离、租约／序号／版本校验、过期／失败／撤销失效及单次入档；内置与外部主持的隔离无头验收均确认模型响应尚未关闭时正文已可见。刷新恢复、1264/390/320px 布局、13 项娱乐场景及7项快捷回复回归通过。
- 证据：`dist/verification/streaming/`。长期 SSH 授权保留；未向正式战役写入测试内容，也未打开正式桌面窗口。

## 当前工程

- 工程目录：`E:\LinChat`
- Windows 客户端：Electron + Vue
- 服务端：Go + SQLite；Windows / Linux x64
- 云端目标：腾讯云 `43.138.138.136`，`chat.linblog.site`
- Radmin 本机网卡：`26.232.226.141`

## 长期 SSH 部署入口（2026-11-25 已建立并验证）

- 用户已要求建立长期 SSH 授权，后续部署使用专用连接：`ssh linchat-cloud "<远程命令>"`，文件传输使用 `scp <文件> linchat-cloud:<目标路径>`。
- 目标：`ubuntu@43.138.138.136:22`。别名保存在本机 `%USERPROFILE%\.ssh\config`，只针对 `linchat-cloud` 生效；采用非交互公钥认证和已有的服务器主机密钥校验。
- 专用 Ed25519 私钥：`%USERPROFILE%\.ssh\linchat_deploy_ed25519`；公钥：同名 `.pub`。私钥只在这台电脑保存，文件 ACL 仅授权当前用户和 SYSTEM，不进入工程或交付附件。
- 公钥标识：`linchat-deploy-desktop-20261125`；指纹：`SHA256:SSKPNskzcPJhS2FuLhDp4Xs7xh4TI+IyGNK8021yffg`。
- 本机已验证密钥对一致、OpenSSH 配置解析正确。服务器 `/home/ubuntu/.ssh/authorized_keys` 已登记以下公钥行，确认只有一条对应授权；`.ssh`／`authorized_keys` 权限为 `0700`／`0600`，均属于 `ubuntu`：

```text
restrict ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOCS3QJFFpJqzsFFC+RfVcbfQdTUt+meiSiHA0XyBVZR linchat-deploy-desktop-20261125
```

- 此授权按用户要求长期保留，不设置自动失效或部署后删除；只有用户要求撤销时才移除对应公钥和本机专用密钥。以后部署清理仅针对暂存文件。
- 已通过真实非交互 SSH 登录、`sudo -n`、独立 SCP 上传／下载及 SHA-256 往返一致性检查。验证时 `linchat.service` 为 `active`，健康接口正常；测试暂存目录已删除，长期授权及本机密钥保留。
- 验证证据：`dist/verification/ssh-persistent/access-results.json`、`transfer-results.json`；交付副本位于 `E:\LinChat\dist\verification\ssh-persistent-20261125\`。证据不包含私钥。

## 2026-11-25 骰型与正视角校正（云端已部署）

- 根据用户截图校正骰子停稳视角：结果面正对镜头、正交等比投影，移除残余俯仰／偏航。D20 正面保持等边三角形与左右对称轮廓；D10／D100 的风筝面尖端朝上，点数移至内心，调整各骰型及长数字字号以避免压住边线。
- 已更新 `https://chat.linblog.site`，刷新聊天页面即可使用。线上 HTML、JS `index-B9OG44Nb.js` 和 CSS `index-BGfj_70s.css` 与本地已验收产物逐字节一致。
- Linux 程序 SHA-256：`bb872cc3f8d13534ee6a87b63d2faa9e7f4b08c058acb94c962e4bda037dd75e`。服务端协议／健康版本仍为 `1.2.0`；本次更新由云端提供的界面生效。
- 更新前完整备份：`/var/backups/linchat/update-20261125-140414-LzlDzh/`。19 张业务表原有记录均保留且无修改，包含 60 条聊天消息、23 条战役事件、4 个账号和4个登录会话；附件／模型密钥摘要一致，SQLite 完整性为 `ok`。
- 类型检查与构建、8 项掷骰集成场景、几何对称／等边／连续收敛检查、深浅色标签边界检查、4 尺寸渲染及普通／减少动态效果捕获通过；机械检测无发现。上线后 6 项公网回归与3项资源／无头检查通过。
- 全程后台／无头验收，未打开应用窗口。服务器暂存目录和隔离预览已清理，长期 SSH 授权保留。证据：`dist/verification/dice-shape/`、`dist/verification/cloud-dice-shape/`、`.impeccable/review/dice-shape/`；交付副本：`E:\LinChat\dist\verification\dice-shape-20261125\`。

## 2026-11-25 掷骰动画（云端已部署）

- 已在当前开发工作树实现骰盘、投掷／翻滚／落定动画与真实服务器结果揭晓，约 820ms；优势／劣势标记保留和舍弃，最多同时绘制六枚骰子，支持查看全部点数。
- 沿用服务器随机数及幂等请求；客户端只负责呈现。慢网停止动画并等待结果，失败可复用原请求重试，Esc 可关闭等待中的弹窗。
- 已覆盖默认深色、自定义浅色面板、320–1264px 布局及减少动态效果；等待提示使用各面板独立的语义琥珀色，实测对比度为 8.07:1／5.14:1。
- 前端类型检查与构建、8 项骰子无头集成场景、13 项娱乐场景回归通过；机械设计检查无发现。动画录屏：`dist/verification/dice/dice-normal.webm` 与 `dice-reduced.webm`；静态与逐帧证据位于 `.impeccable/review/dice/`。
- 动效复核修正了等待状态的语义色，最终结论为 `ship`，范围为提供的逐帧证据和深／浅色等待状态。交付副本位于 `E:\LinChat\dist\verification\dice-animation-20261125\`。
- 已部署至 `https://chat.linblog.site`。当前已打开的聊天页按 `Ctrl + R` 刷新，即可从「娱乐模式 → 掷骰」使用新动画。
- 部署程序 SHA-256：`8bd6c0157b9ad34b588a3f881d0aaffe462ebaf1273141fd79735fe2caa5be03`；构建前逐项确认源码与前端产物均符合已验收输入清单。服务端协议／健康版本仍为 `1.2.0`。
- 更新前已完整备份旧程序与数据：`/var/backups/linchat/update-20261125-134135-ZTB1ZJ/`。19 张业务表原有记录逐项一致，包含 52 条消息、4 个账号、4 个登录会话、1 个战役、2 张角色卡；附件和模型密钥文件摘要一致，SQLite 完整性为 `ok`。
- 线上 HTML、JS `index-CdDdGTAK.js` 与 CSS `index-B2TWY0MD.css` 和已验收构建逐字节一致。6 项公网检查与 3 项上线资源／无头检查通过。登录后的掷骰行为使用此前的隔离战役验收，公网验证未向正式战役写入测试记录。
- 服务器暂存文件已清理，长期 SSH 授权及本机密钥保留。部署证据：`dist/verification/cloud-dice/`；交付副本：`E:\LinChat\dist\verification\cloud-dice-20261125\`。

## 2026-11-25 云端外观补部署（1.2.2 界面）

- 已将包含深色界面、自定义配色及新版桌面更新提示的 1.2.2 发行服务端部署至 `https://chat.linblog.site`。聊天和娱乐模式页面现在由云端提供新版资源；已打开的旧页面需要刷新一次。
- 使用已发布且已验收的 Linux 程序，SHA-256：`b48bab8af2f91841efe355ad933e440777ef65045dd3038192247b729571c82e`。服务端协议／健康版本仍为 `1.2.0`，本次构建通过二进制和前端资源摘要确认。
- 更新前停止服务并备份完整数据和旧程序：`/var/backups/linchat/update-20261125-125758-TIgx3W/`。
- 19 张业务表原有记录逐项对比一致，包含 4 个账号（含 1 个服务账号）、5 个会话、12 条成员关系、50 条消息、4 个登录会话、1 个战役、2 张角色卡、11 条战役事件。附件与模型密钥文件摘要一致，SQLite 完整性为 `ok`。
- 线上 JS `index-C_s321Yv.js` 与 CSS `index-CHkUr-1v.css` 和已验收构建逐字节一致。6 项公网检查与 4 项无头外观检查通过，包含默认深色、切换配色后刷新保留、390px 布局及无未捕获错误。
- 全程后台／无头执行，未弹出或聚焦应用窗口。证据：`dist/verification/cloud-appearance/`，交付副本：`E:\LinChat\dist\verification\cloud-appearance-1.2.2\`。
- 已撤销本次 `linchat-appearance-20261125` 临时授权，保留其他 SSH 授权；服务器暂存目录及本地临时密钥均已清理。

## 1.2.2 应用内自动更新与深色配色

- 已发布：[LinChat 1.2.2](https://github.com/Lin-6499/ChatForPlay/releases/tag/v1.2.2)，发行源码提交 `0b32c32`。9 个附件的大小和 SHA-256 均经 GitHub 确认。
- 安装版采用 `electron-updater 6.8.11`：后台下载、SHA-512 校验、下载进度及失败重试；下载完成后点击“重启并更新”，通过 NSIS 静默安装并自动重启。
- 安装路径固定为当前程序所在目录。首次安装保留可选目录向导；便携版与开发环境提供下载安装版入口。
- 重启前再次校验缓存安装包并等待内置服务有序退出；超时中止更新。重启后恢复原本的本机／Radmin 主机和聊天连接。
- 合并已验证的深色外观与自定义配色：墨色、夜蓝、暖灰、黑曜预设，以及强调色／背景色／面板色编辑。
- 真实 NSIS 升级夹具采用独立 appId、进程名及数据目录，完成中文／空格自选目录升级、损坏下载拒绝、静默安装、自动重启、本机服务恢复、登录／聊天／战役／配色保留及安装后 app.asar 逐字节对比。6 项通过，结果：`dist/verification/auto-update-results.json`。
- Go 测试与 vet、13 项 JS／配色测试、13 项 Electron 回归和 13 项娱乐场景通过。打包后的正式 EXE 已通过 GitHub 版本检查和云端 HTTPS 登录页检查。
- 正式安装包：107,514,125 字节；SHA-256：`4723703e08d451587d90c67cca5b7e3b1bdbbb86986192b442d42c46cee733d1`。`latest.yml` 中的版本、文件大小及 SHA-512 已与最终安装包交叉校验。
- 正式 app.asar 的 15 个桌面文件与源码一致、内置服务端与验证二进制一致、测试入口未打包。结果：`dist/verification/build-binding.json`。
- 已使用 Windows PowerShell 5.1 从正式地址完整下载安装包，大小与 SHA-256 校验通过。公开 `latest.yml`、blockmap 及全部 9 个附件再次校验通过；旧客户端收到更新，当前版本不重复提醒。结果：`dist/verification/published-update-check.json`。
- 本机从 **1.2.0 静默升级至 1.2.2** 并重新打开，安装位置仍为 `C:\Users\Administrator\AppData\Local\Programs\linchat-desktop\LinChat.exe`。安装期间原用户目录 88 个文件逐一 SHA-256 对比完全一致；EXE、app.asar、内置服务端和更新配置与最终构建一致。结果：`dist/verification/installer-results.json`。
- 本机升级前完整用户数据备份：自动更新工作树的 `dist/verification/profile-before-1.2.2-20261125-113841`，交付副本位于 `E:\LinChat\dist\verification\auto-update-1.2.2`。
- 实际安装版通过独立验证配置访问公开更新源（`autoUpdate: true`、当前／最新版本均为 `1.2.2`、无错误），并连接云端 HTTPS 登录页。结果：`dist/verification/installed-check-results.json`。
- 1.2.1 及更早版本需先升级一次 1.2.2，之后可在应用内自动更新；本机这次引导升级已代为完成。
- 云端及服务端协议版本仍为 1.2.0；本次桌面更新不要求云端先升级。旧云端页面的 `openUpdate()` 入口兼容新版客户端，也可从托盘查看下载进度和执行重启。

## 1.2.1 自定义安装位置（桌面更新）

- 已发布：[LinChat 1.2.1](https://github.com/Lin-6499/ChatForPlay/releases/tag/v1.2.1)，发行源码提交 `af640af`；7 个发行附件的大小和 SHA-256 均经 GitHub 确认。
- Windows 安装器改为当前用户安装向导，支持“浏览”或输入目标目录；升级时默认显示已登记的安装位置。
- `一键安装使用.cmd` 始终进入同版／新版安装向导，支持重新选择目录；`启动LinChat.cmd` 从 Windows 安装登记中识别程序路径并直接启动已安装的版本。
- 安装器完成页控制是否运行；取消安装正常退出。安装包缓存位于 `%LOCALAPPDATA%\LinChat\downloads`，数据位于 `%APPDATA%\linchat-desktop`。
- 安装包 `LinChat-Setup-1.2.1.exe`：107,176,264 字节；SHA-256：`7e31c054f54344016778fa62b06ead27d74ccf1971b56794a4fa5bd7cf7ce762`。
- 已构建安装版与便携版；真实安装向导显示目录输入框、“浏览”和原安装路径。用户关闭了本次向导，本机安装版本仍为 1.2.0。截图和状态：`dist/verification/installer-1.2.1/installer-directory.png`、`custom-installer-results.json`。
- 打包 EXE 1.2.1 已通过启动、官方更新接口及云端 HTTPS 登录页检查。结果：`dist/verification/installer-1.2.1/packaged-check-results.json`。
- 已使用 Windows PowerShell 5.1 从正式地址完整下载 1.2.1 安装包，大小与 SHA-256 校验通过；正式更新接口向 1.2.0 提示升级，1.2.1 不重复提醒。结果：`dist/verification/installer-1.2.1/download-results.json`、`published-update-check.json`。
- 本版更新桌面安装流程；内置及云端服务端为 1.2.0。

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
- 已部署深色界面、自定义配色、掷骰动画及骰型／正视角校正；服务端健康接口保留协议版本 `1.2.0`。
- Linux 二进制 SHA-256：`bb872cc3f8d13534ee6a87b63d2faa9e7f4b08c058acb94c962e4bda037dd75e`。

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
