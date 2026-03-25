# LAN Chat System - Install Script
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  LAN Chat System - Install Script" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check Go
Write-Host "[1/5] Checking Go..." -ForegroundColor Yellow
try {
    $goVersion = go version
    Write-Host "[OK] $goVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Go not found. Install from https://go.dev/dl/" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Check Node.js
Write-Host ""
Write-Host "[2/5] Checking Node.js..." -ForegroundColor Yellow
try {
    $nodeVersion = node --version
    Write-Host "[OK] Node.js $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Node.js not found. Install from https://nodejs.org/" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Check MySQL
Write-Host ""
Write-Host "[3/5] Checking MySQL..." -ForegroundColor Yellow
try {
    $mysqlVersion = mysql --version
    Write-Host "[OK] $mysqlVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] MySQL not found. Install from https://dev.mysql.com/downloads/mysql/" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Init database
Write-Host ""
Write-Host "[4/5] Initializing database..." -ForegroundColor Yellow
mysql -u root -p123456 < backend/database.sql
if ($LASTEXITCODE -eq 0) {
    Write-Host "[OK] Database initialized" -ForegroundColor Green
} else {
    Write-Host "[WARNING] Database init may have failed" -ForegroundColor Yellow
}

# Install backend deps
Write-Host ""
Write-Host "[5/5] Installing backend dependencies..." -ForegroundColor Yellow
Set-Location backend
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Backend dependency install failed" -ForegroundColor Red
    Set-Location ..
    Read-Host "Press Enter to exit"
    exit 1
}
Set-Location ..
Write-Host "[OK] Backend dependencies installed" -ForegroundColor Green

# Install frontend deps
Write-Host ""
Write-Host "Installing frontend dependencies..." -ForegroundColor Yellow
Set-Location frontend
npm install
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Frontend dependency install failed" -ForegroundColor Red
    Set-Location ..
    Read-Host "Press Enter to exit"
    exit 1
}
Set-Location ..
Write-Host "[OK] Frontend dependencies installed" -ForegroundColor Green

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Install Complete!" -ForegroundColor Green
Write-Host "  Run start.ps1 to start the system" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Read-Host "Press Enter to exit"