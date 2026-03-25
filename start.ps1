# LAN Chat System - Start Script
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  LAN Chat System - Start Script" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Init database
Write-Host "[1/3] Initializing database..." -ForegroundColor Yellow
mysql -u root -p123456 < backend/database.sql 2>$null
Write-Host "[OK] Database ready" -ForegroundColor Green

# Start backend
Write-Host ""
Write-Host "[2/3] Starting backend server..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$PWD\backend'; go run main.go" -WindowStyle Normal
Write-Host "[OK] Backend starting..." -ForegroundColor Green

Write-Host ""
Write-Host "Waiting for backend to start..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Start frontend
Write-Host ""
Write-Host "[3/3] Starting frontend server..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$PWD\frontend'; npm run dev" -WindowStyle Normal
Write-Host "[OK] Frontend starting..." -ForegroundColor Green

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  All Services Started!" -ForegroundColor Green
Write-Host ""
Write-Host "  Frontend: http://localhost:34115" -ForegroundColor White
Write-Host "  Backend:  http://localhost:8080" -ForegroundColor White
Write-Host ""
Write-Host "  Check backend console for LAN IP" -ForegroundColor Yellow
Write-Host "  to connect from mobile devices" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan
Read-Host "Press Enter to exit"
