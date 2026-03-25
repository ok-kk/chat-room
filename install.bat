@echo off
echo ========================================
echo   LAN Chat System - Install Script
echo ========================================
echo.

echo [1/5] Checking Go environment...
go version >nul 2>&1
if ERRORLEVEL 1 (
    echo [ERROR] Go not found. Please install Go 1.21+
    echo Download: https://go.dev/dl/
    pause
    exit /b 1
)
echo [OK] Go is ready

echo.
echo [2/5] Checking Node.js environment...
node --version >nul 2>&1
if ERRORLEVEL 1 (
    echo [ERROR] Node.js not found. Please install Node.js 18+
    echo Download: https://nodejs.org/
    pause
    exit /b 1
)
echo [OK] Node.js is ready

echo.
echo [3/5] Checking MySQL environment...
mysql --version >nul 2>&1
if ERRORLEVEL 1 (
    echo [ERROR] MySQL not found. Please install MySQL 8.0+
    echo Download: https://dev.mysql.com/downloads/mysql/
    pause
    exit /b 1
)
echo [OK] MySQL is ready

echo.
echo [4/5] Initializing database...
mysql -u root -p123456 < backend\database.sql
if ERRORLEVEL 1 (
    echo [WARNING] Database init may have failed, please check MySQL connection
) else (
    echo [OK] Database initialized
)

echo.
echo [5/5] Installing backend dependencies...
cd backend
go mod tidy
if ERRORLEVEL 1 (
    echo [ERROR] Backend dependency install failed
    cd ..
    pause
    exit /b 1
)
cd ..
echo [OK] Backend dependencies installed

echo.
echo Installing frontend dependencies...
cd frontend
call npm install
if ERRORLEVEL 1 (
    echo [ERROR] Frontend dependency install failed
    cd ..
    pause
    exit /b 1
)
cd ..
echo [OK] Frontend dependencies installed

echo.
echo ========================================
echo   Install Complete!
echo   Run start.bat to start the system
echo ========================================
echo.
pause