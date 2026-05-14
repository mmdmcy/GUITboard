@echo off
setlocal EnableExtensions EnableDelayedExpansion

if /I "%~1"=="--current-console" shift

cd /d "%~dp0"
set "GUITBOARD_NO_ALT_SCREEN=1"
set "GUITBOARD_PLAIN_TERMINAL=1"
set "PORTUI_INTERACTIVE=1"
set "NO_COLOR=1"
set "GUITBOARD_STARTUP_LOG=%TEMP%\GUITboard-startup.log"
del "%GUITBOARD_STARTUP_LOG%" >nul 2>nul

if exist "%~dp0dist\GUITboard.exe" (
    "%~dp0dist\GUITboard.exe" %*
    set "GUITBOARD_EXIT=!ERRORLEVEL!"
) else (
    where go.exe >nul 2>nul
    if not errorlevel 1 (
        go run ./cmd/guitboard %*
        set "GUITBOARD_EXIT=!ERRORLEVEL!"
    ) else (
        echo GUITboard needs Go to run from source, and no local dist\GUITboard.exe was found.
        echo Install Go with: winget install -e --id GoLang.Go
        set "GUITBOARD_EXIT=127"
    )
)

if not "!GUITBOARD_EXIT!"=="0" pause
exit /b !GUITBOARD_EXIT!
