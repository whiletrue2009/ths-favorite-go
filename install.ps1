# ths 一键安装（Windows PowerShell）：下载最新版到 ~\.local\bin
$ErrorActionPreference = "Stop"

$Repo = "whiletrue2009/ths-favorite-go"
$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  "ARM64" { "arm64" }
  default { Write-Error "不支持的 CPU 架构: $($env:PROCESSOR_ARCHITECTURE)"; exit 1 }
}

Write-Host ">> 查询最新版本…"
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Asset = $Release.assets | Where-Object { $_.name -like "ths_*_windows_$Arch.zip" } | Select-Object -First 1
if (-not $Asset) { Write-Error "未找到 windows/$Arch 的发布包"; exit 1 }

$Tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP ([System.Guid]::NewGuid()))
Write-Host ">> 下载 $($Asset.browser_download_url)"
$ZipPath = Join-Path $Tmp "ths.zip"
Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $ZipPath
Expand-Archive -Path $ZipPath -DestinationPath $Tmp -Force

$BinDir = Join-Path $env:USERPROFILE ".local\bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
Move-Item -Force (Join-Path $Tmp "ths.exe") (Join-Path $BinDir "ths.exe")

Write-Host ">> 安装完成: $BinDir\ths.exe"
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$BinDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$BinDir", "User")
  Write-Host "!! 已把 $BinDir 加入用户 PATH，请重开终端生效"
}
Write-Host ">> 下一步：运行 ths login 登录，然后对 AI 说「帮我看看自选股」"
