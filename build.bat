@echo off
echo ========================================
echo   LAN Chat - Build Desktop App
echo ========================================
echo.

echo [1/4] Check Wails...
where wails >nul 2>&1
if ERRORLEVEL 1 (
    echo [INFO] Installing Wails CLI...
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    if ERRORLEVEL 1 (
        echo [ERROR] Failed to install Wails
        pause
        exit /b 1
    )
)
echo [OK] Wails ready

echo.
echo [2/4] Install frontend deps...
cd frontend
call npm install
if ERRORLEVEL 1 (
    echo [ERROR] npm install failed
    cd ..
    pause
    exit /b 1
)
cd ..
echo [OK] Deps installed

echo.
echo [3/4] Build frontend...
cd frontend
call npm run build
if ERRORLEVEL 1 (
    echo [ERROR] Frontend build failed
    cd ..
    pause
    exit /b 1
)
cd ..
echo [OK] Frontend built

echo.
echo [4/4] Build desktop app...
wails build -clean
if ERRORLEVEL 1 (
    echo [ERROR] Wails build failed
    pause
    exit /b 1
)

echo.
echo ========================================
echo   Build Complete!
echo   App: build\bin\lan-chat.exe
echo ========================================
echo.
explorer build\bin
pause