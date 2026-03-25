@echo off
echo ========================================
echo   Quick Start - Skip Install
echo ========================================
echo.

echo Starting backend...
cd backend
start "LAN Chat Backend" cmd /k "go run gateway\main.go"
cd ..

timeout /t 5 /nobreak >nul

echo Starting frontend...
cd frontend
start "LAN Chat Frontend" cmd /k "npm run dev"
cd ..

echo.
echo ========================================
echo   Started!
echo   Frontend: http://localhost:3000
echo   Backend:  http://localhost:8080
echo ========================================
pause