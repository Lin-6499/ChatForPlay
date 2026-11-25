# 执行与验收约定

- 用户要求修改及验收全程后台进行，不弹出、聚焦或切换应用窗口，也不打开可见预览。
- 界面验收使用无头浏览器（`headless: true`）；不要启动 Electron 桌面窗口进行 smoke test。
- Windows 测试脚本启动后台服务和子进程时设置 `windowsHide: true`。
- 测试使用隔离数据目录，不改变正在使用的账号、战役或云端配置。
- 运行 Electron／安装器验收前确认其真正以隐藏方式运行。现有 `test:desktop`、`test:auto-update` 和 `scripts/packaged-check.cjs` 会显示窗口，不能直接用于无人值守验收。
- 确实需要可见窗口时，先征得用户同意。
