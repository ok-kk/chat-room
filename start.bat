@echo off
echo ========================================
echo   LAN Chat System - Start Script
echo ========================================
echo.

echo [1/3] Initializing database...
mysql -u root -p123456 < backend\database.sql 2>nul
echo [OK] Database ready

echo.
echo [2/3] Starting backend server...
cd backend
start "LAN Chat Backend" cmd /k "go run main.go"
cd ..
echo [OK] Backend starting...

echo.
echo Waiting for backend to start...
timeout /t 5 /nobreak >nul

echo.
echo [3/3] Starting frontend server...
cd frontend
start "LAN Chat Frontend" cmd /k "npm run dev"
cd ..
echo [OK] Frontend starting...

echo.
echo ========================================
echo   All Services Started!
echo.
echo   Frontend: http://localhost:34115
echo   Backend:  http://localhost:8080
echo.
echo   Check backend console for LAN IP
echo   to connect from mobile devices
echo ========================================
echo.
pause
