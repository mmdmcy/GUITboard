@echo off
setlocal

if "%~1"=="" (
    call "%~dp0GUITboard.cmd"
) else (
    powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0\.portui-runtime\portui.ps1" -ManifestDir "%~dp0portui" %*
)

exit /b %ERRORLEVEL%
