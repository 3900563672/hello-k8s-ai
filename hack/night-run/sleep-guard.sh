#!/bin/bash
# sleep-guard.sh - WSL wrapper for Windows powercfg sleep guard (night-run)
# Usage: bash hack/night-run/sleep-guard.sh [status|on|off]
#   status:  query current AC standby/hibernate timeout (no admin needed)
#   on:      disable AC sleep/hibernate (UAC prompt, requires user click)
#   off:     restore saved values (UAC prompt, requires user click)
set -u

ACTION="${1:-status}"
PS1_UNC='\\wsl.localhost\Ubuntu\root\hello-k8s-ai\hack\night-run\sleep-guard.ps1'
LOG_WSL="/mnt/c/Users/hh/AppData/Local/Temp/sleep-guard.log"

case "$ACTION" in
  status)
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PS1_UNC" status 2>&1
    ;;
  on|off)
    if [ -f "$LOG_WSL" ]; then rm -f "$LOG_WSL"; fi
    powershell.exe -NoProfile -Command "Start-Process -FilePath 'powershell.exe' -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','$PS1_UNC','$ACTION' -Verb RunAs -WindowStyle Hidden" >/dev/null 2>&1
    echo "UAC prompt sent for action=$ACTION; waiting for elevated script to finish..."
    for _ in $(seq 1 30); do
      if [ -f "$LOG_WSL" ]; then
        sleep 2
        cat "$LOG_WSL"
        echo "---"
        powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PS1_UNC" status 2>&1
        exit 0
      fi
      sleep 2
    done
    echo "ERROR: no log after 60s (UAC not confirmed or elevated script failed)"
    exit 1
    ;;
  *)
    echo "usage: $0 [status|on|off]" >&2
    exit 2
    ;;
esac