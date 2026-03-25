#!/bin/bash

echo "========================================"
echo "  局域网即时通信系统 - 启动脚本"
echo "========================================"
echo ""

# 检查MySQL是否运行
echo "[1/4] 检查MySQL服务..."
if ! command -v mysql &> /dev/null; then
    echo "[错误] 未找到MySQL，请确保MySQL已安装并添加到PATH"
    exit 1
fi
echo "[OK] MySQL已就绪"

# 初始化数据库
echo ""
echo "[2/4] 初始化数据库..."
mysql -u root -p123456 < backend/database.sql
echo "[OK] 数据库初始化完成"

# 启动后端
echo ""
echo "[3/4] 启动后端服务..."
cd backend
go mod tidy
go run gateway/main.go &
BACKEND_PID=$!
cd ..
echo "[OK] 后端服务启动中 (PID: $BACKEND_PID)"

# 等待后端启动
sleep 3

# 启动前端
echo ""
echo "[4/4] 启动前端服务..."
cd frontend
npm install
npm run dev &
FRONTEND_PID=$!
cd ..
echo "[OK] 前端服务启动中 (PID: $FRONTEND_PID)"

echo ""
echo "========================================"
echo "  服务启动完成!"
echo ""
echo "  前端访问: http://localhost:3000"
echo "  后端API:  http://localhost:8080"
echo ""
echo "  手机扫码访问请查看后端控制台输出的IP地址"
echo ""
echo "  按 Ctrl+C 停止所有服务"
echo "========================================"

# 捕获退出信号
trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null" EXIT

# 等待
wait