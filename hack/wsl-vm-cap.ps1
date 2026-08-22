# 读取 .wslconfig 的 [wsl2] memory=（GB），输出 MB；无配置或解析失败时输出默认 16384（16GB）。
# 供 hack/doctor.sh 动态计算 WSL VM 内存 WARN 阈值，避免硬编码 12GB 漂移（2026-08-22）。
# 用法: powershell.exe -NoProfile -ExecutionPolicy Bypass -File hack/wsl-vm-cap.ps1
$ErrorActionPreference = 'SilentlyContinue'
$cfg = Join-Path ([Environment]::GetFolderPath('UserProfile')) '.wslconfig'
$memLine = Get-Content $cfg | Where-Object { $_ -match '^\s*memory\s*=\s*([0-9.]+)\s*[gG][bB]' } | Select-Object -Last 1
if ($memLine -match '^\s*memory\s*=\s*([0-9.]+)\s*[gG][bB]') {
    Write-Output ([double]$Matches[1] * 1024)
} else {
    Write-Output 16384
}