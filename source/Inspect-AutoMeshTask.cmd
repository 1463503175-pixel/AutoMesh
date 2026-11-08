@echo off
setlocal
set "SCRIPT=%~dp0Inspect-AutoMeshTask.ps1"
if "%~1"=="" (
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%"
) else (
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" -DataDir "%~1"
)
pause
