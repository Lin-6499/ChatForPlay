# 项目操作约定

- 用户要求：后续修改和验收不要弹出应用窗口、抢占焦点或自动打开正式客户端。
- Web 界面验收使用无头浏览器；构建、部署和检查在后台执行。
- 运行 Electron／安装器验收前确认其真正以隐藏方式运行。现有 `test:desktop`、`test:auto-update` 和 `scripts/packaged-check.cjs` 会显示窗口，不能直接用于无人值守验收。
- 确实需要可见窗口时，先征得用户同意。
- 云端长期部署使用 SSH 别名 `linchat-cloud`；授权状态见 `DEPLOYMENT.md`。专用密钥 `%USERPROFILE%\.ssh\linchat_deploy_ed25519` 属于用户要求保留的长期授权，不要在部署后按临时凭据流程删除；用户明确要求撤销时再移除。
