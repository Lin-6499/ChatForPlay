@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\install-and-start.ps1" -Reinstall
if errorlevel 1 (
    echo.
    echo Installation did not finish. You can also download the installer here:
    echo https://github.com/Lin-6499/ChatForPlay/releases/latest
    pause
    exit /b 1
)
exit /b 0
