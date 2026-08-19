# sleep-guard.ps1 - night-run sleep guard (powercfg on/off/status)
# Usage: powershell -File sleep-guard.ps1 status|on|off
# Output is ASCII-friendly for WSL parsing: standby_ac=<min> hibernate_ac=<min> guard=on|off
param([Parameter(Mandatory=$true)][ValidateSet('status','on','off')]$Action)
$ErrorActionPreference = 'Stop'
$log = Join-Path $env:TEMP 'sleep-guard.log'
$stateFile = Join-Path $env:LOCALAPPDATA 'night-run-sleep-guard.json'

function Log([string]$msg) {
    ("{0:o} {1}" -f (Get-Date).ToUniversalTime(), $msg) | Out-File -Append -Encoding utf8 $log
}

function Get-AcTimeout([string]$guid) {
    $out = powercfg /query SCHEME_CURRENT SUB_SLEEP $guid
    $line = ($out | Select-String '当前交流电源设置索引' | Select-Object -First 1).ToString()
    $hex = [regex]::Match($line, '0x[0-9a-fA-F]+').Value
    if (-not $hex) { throw "cannot parse AC value for $guid" }
    return [Convert]::ToInt32($hex.Substring(2), 16) / 60
}

function Set-StandbyTimeout {
    [CmdletBinding(SupportsShouldProcess = $true)]
    param([int]$Minutes)
    if ($PSCmdlet.ShouldProcess("standby-timeout-ac", "set to $Minutes")) {
        powercfg /change standby-timeout-ac $Minutes
        if ($LASTEXITCODE -ne 0) { throw "powercfg /change standby-timeout-ac $Minutes failed" }
    }
}

function Set-HibernateTimeout {
    [CmdletBinding(SupportsShouldProcess = $true)]
    param([int]$Minutes)
    if ($PSCmdlet.ShouldProcess("hibernate-timeout-ac", "set to $Minutes")) {
        powercfg /change hibernate-timeout-ac $Minutes
        if ($LASTEXITCODE -ne 0) { throw "powercfg /change hibernate-timeout-ac $Minutes failed" }
    }
}

switch ($Action) {
    'status' {
        $sb = Get-AcTimeout 'STANDBYIDLE'
        $hb = Get-AcTimeout 'HIBERNATEIDLE'
        $guard = if ($sb -eq 0 -and $hb -eq 0) { 'on' } else { 'off' }
        Write-Output ("standby_ac={0} hibernate_ac={1} guard={2}" -f $sb, $hb, $guard)
    }
    'on' {
        $sb = Get-AcTimeout 'STANDBYIDLE'
        $hb = Get-AcTimeout 'HIBERNATEIDLE'
        @{ standbyAc = $sb; hibernateAc = $hb } | ConvertTo-Json | Set-Content -Encoding utf8 $stateFile
        Set-StandbyTimeout 0
        Set-HibernateTimeout 0
        Log ("on: saved {0}/{1} min, set 0/0" -f $sb, $hb)
        Write-Output 'on: sleep guard enabled (standby/hibernate AC = 0)'
    }
    'off' {
        $sb = 15; $hb = 180
        if (Test-Path $stateFile) {
            $saved = Get-Content $stateFile | ConvertFrom-Json
            $sb = [int]$saved.standbyAc; $hb = [int]$saved.hibernateAc
        }
        Set-StandbyTimeout $sb
        Set-HibernateTimeout $hb
        Log ("off: restored {0}/{1} min" -f $sb, $hb)
        Write-Output ("off: restored standby/hibernate AC = {0}/{1} min" -f $sb, $hb)
    }
}