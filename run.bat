@echo off
echo ========================================
echo   LAN Chat - Production Start
echo ========================================
echo.

echo [1/3] Building frontend...
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
echo [2/3] Initializing database...
mysql -u root -p111111 < backend\database.sql 2>nul
echo [OK] Database ready

echo.
echo [3/3] Starting server...
echo.
echo ========================================
echo   Server starting on port 8080
echo.
echo   Local:  http://localhost:8080
echo.
echo   Check console for LAN IP address
echo   then visit that address from phone
echo ========================================
echo.
cd backend
go run main.go
