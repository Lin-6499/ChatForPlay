@echo off
cd /d "%~dp0"
if exist "dist\desktop\win-unpacked\LinChat.exe" (
    start "" "dist\desktop\win-unpacked\LinChat.exe"
) else (
    echo LinChat has not been built. Run npm run package first.
    pause
)
