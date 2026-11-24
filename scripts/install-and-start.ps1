[CmdletBinding()]
param(
    [switch]$DownloadOnly,
    [switch]$ForceDownload,
    [switch]$NoLaunch
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$OutputEncoding = [Console]::OutputEncoding
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$projectRoot = Split-Path -Parent $PSScriptRoot
$releasePage = 'https://github.com/Lin-6499/ChatForPlay/releases/latest'

function Write-Step([string]$Text) {
    Write-Host "[LinChat] $Text" -ForegroundColor Cyan
}

function Get-InstalledApp {
    $paths = New-Object 'System.Collections.Generic.List[string]'
    $registryRoots = @(
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    foreach ($registryRoot in $registryRoots) {
        $entries = @(Get-ItemProperty -Path $registryRoot -ErrorAction SilentlyContinue |
            Where-Object { $_.DisplayName -eq 'LinChat' -and $_.InstallLocation })
        foreach ($entry in $entries) {
            $paths.Add((Join-Path $entry.InstallLocation 'LinChat.exe'))
        }
    }
    $paths.Add((Join-Path $env:LOCALAPPDATA 'Programs\LinChat\LinChat.exe'))
    $paths.Add((Join-Path $env:LOCALAPPDATA 'Programs\linchat-desktop\LinChat.exe'))
    foreach ($candidate in ($paths | Select-Object -Unique)) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            $file = Get-Item -LiteralPath $candidate
            $versionText = $file.VersionInfo.ProductVersion
            if ($file.VersionInfo.ProductName -eq 'LinChat' -and $versionText -match '^\d+\.\d+\.\d+(\.\d+)?$') {
                $normalizedVersion = $versionText -replace '^(\d+\.\d+\.\d+)$', '$1.0'
                return [pscustomobject]@{ Path = $file.FullName; Version = [version]$normalizedVersion }
            }
        }
    }
    return $null
}

function Test-Installer([string]$Path, $Manifest) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    if ((Get-Item -LiteralPath $Path).Length -ne [long]$Manifest.bytes) { return $false }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -eq $Manifest.sha256
}

function Download-Installer($Manifest, [string]$Destination) {
    $partial = "$Destination.partial"
    $lastError = $null
    # Use the Windows proxy settings first, then direct transport if the proxy fails.
    foreach ($mode in @('system', 'direct')) {
        $response = $null
        $inputStream = $null
        $outputStream = $null
        try {
            if (Test-Path -LiteralPath $partial) { Remove-Item -LiteralPath $partial -Force }
            Write-Step "正在下载安装包（约 $([math]::Ceiling([long]$Manifest.bytes / 1MB)) MB），请保持窗口打开……"
            $request = [Net.HttpWebRequest]::Create([uri]$Manifest.url)
            $request.UserAgent = 'LinChat-Windows-Installer/1.0'
            $request.AllowAutoRedirect = $true
            $request.Timeout = 30000
            $request.ReadWriteTimeout = 60000
            if ($mode -eq 'direct') { $request.Proxy = $null }
            $response = $request.GetResponse()
            if ($response.ResponseUri.Scheme -ne 'https') { throw '下载地址未使用 HTTPS。' }
            $inputStream = $response.GetResponseStream()
            $outputStream = [IO.File]::Open($partial, [IO.FileMode]::Create, [IO.FileAccess]::Write, [IO.FileShare]::None)
            $buffer = New-Object byte[] (256KB)
            $total = [long]0
            $lastPercent = -10
            $watch = [Diagnostics.Stopwatch]::StartNew()
            while (($count = $inputStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
                $total += $count
                if ($total -gt [long]$Manifest.bytes) { throw '下载内容大小超过发行记录，请重新下载项目。' }
                if ($watch.Elapsed.TotalMinutes -gt 15) { throw '下载超时，请检查网络后再次双击安装。' }
                $outputStream.Write($buffer, 0, $count)
                $percent = [math]::Floor(100 * $total / [long]$Manifest.bytes)
                if ($percent -ge $lastPercent + 10) {
                    Write-Host "  下载进度：$percent%"
                    $lastPercent = $percent
                }
            }
            $outputStream.Dispose(); $outputStream = $null
            $inputStream.Dispose(); $inputStream = $null
            $response.Dispose(); $response = $null
            Write-Step '正在校验安装包……'
            if (-not (Test-Installer $partial $Manifest)) {
                throw '安装包校验失败，文件可能未下载完整。'
            }
            Move-Item -LiteralPath $partial -Destination $Destination -Force
            return
        } catch {
            $lastError = $_
            if ($mode -eq 'system') { Write-Step '当前网络方式下载失败，正在尝试直连……' }
        } finally {
            if ($outputStream) { $outputStream.Dispose() }
            if ($inputStream) { $inputStream.Dispose() }
            if ($response) { $response.Dispose() }
            if (Test-Path -LiteralPath $partial) { Remove-Item -LiteralPath $partial -Force }
        }
    }
    throw $lastError
}

try {
    Write-Host ''
    Write-Host 'LinChat 一键安装与启动' -ForegroundColor Green
    Write-Host '首次运行自动安装，之后双击即可打开。'
    Write-Host ''
    if (-not [Environment]::Is64BitOperatingSystem) {
        throw '此安装包需要 64 位 Windows。'
    }
    $manifestPath = Join-Path $projectRoot 'desktop-release.json'
    if (-not (Test-Path -LiteralPath $manifestPath)) {
        throw '缺少发行信息 desktop-release.json，请先将下载的项目 ZIP 完整解压。'
    }
    $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($manifest.version -notmatch '^\d+\.\d+\.\d+$' -or
        $manifest.filename -notmatch '^LinChat-Setup-\d+\.\d+\.\d+\.exe$' -or
        $manifest.sha256 -notmatch '^[a-fA-F0-9]{64}$' -or [long]$manifest.bytes -le 0) {
        throw '发行信息格式无效，请重新下载项目。'
    }
    $expectedURL = "https://github.com/Lin-6499/ChatForPlay/releases/download/v$($manifest.version)/$($manifest.filename)"
    if ($manifest.url -cne $expectedURL) { throw '发行下载地址无效。' }
    $targetVersion = [version]"$($manifest.version).0"
    $installed = Get-InstalledApp
    if ($installed -and $installed.Version -ge $targetVersion -and -not $DownloadOnly) {
        Write-Step "已安装 LinChat $($installed.Version)，正在打开……"
        if (-not $NoLaunch) { Start-Process -FilePath $installed.Path -WorkingDirectory (Split-Path $installed.Path) }
        exit 0
    }

    $cacheDirectory = Join-Path $env:LOCALAPPDATA 'LinChat\downloads'
    New-Item -ItemType Directory -Path $cacheDirectory -Force | Out-Null
    $cachedInstaller = Join-Path $cacheDirectory $manifest.filename
    $installer = $null
    if (-not $ForceDownload) {
        $candidates = @(
            (Join-Path $projectRoot "dist\desktop\$($manifest.version)\$($manifest.filename)"),
            (Join-Path $projectRoot "dist\desktop\$($manifest.filename)"),
            (Join-Path $projectRoot $manifest.filename),
            $cachedInstaller
        )
        foreach ($candidate in $candidates) {
            if (Test-Installer $candidate $manifest) { $installer = $candidate; break }
        }
    }
    if (-not $installer) {
        Download-Installer $manifest $cachedInstaller
        $installer = $cachedInstaller
    }
    Write-Step "安装包校验通过：$($manifest.filename)"
    if ($DownloadOnly) {
        Write-Step "已下载到：$installer"
        exit 0
    }

    Write-Step '正在为当前 Windows 用户安装，并创建桌面快捷方式……'
    $process = Start-Process -FilePath $installer -ArgumentList @('/S', '/currentuser') -PassThru -Wait
    if ($process.ExitCode -ne 0) { throw "安装程序返回错误代码 $($process.ExitCode)。" }
    $installed = Get-InstalledApp
    if (-not $installed -or $installed.Version -lt $targetVersion) {
        throw '未找到已安装的 LinChat。请打开发行页面，直接运行安装包。'
    }
    Write-Step "安装完成：$($installed.Path)"
    if (-not $NoLaunch) {
        Start-Process -FilePath $installed.Path -WorkingDirectory (Split-Path $installed.Path)
        Write-Step 'LinChat 已启动。云端地址：https://chat.linblog.site'
    }
    exit 0
} catch {
    Write-Host ''
    Write-Host "安装未完成：$($_.Exception.Message)" -ForegroundColor Red
    Write-Host "直接下载安装包：$releasePage"
    exit 1
}
