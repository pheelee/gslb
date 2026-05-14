#!/bin/bash
# GSLB Dev Server Startup Script
# Starts backend on port 8090 and frontend on port 3000 with proxy

set -e

echo "🚀 Starting GSLB Development Environment..."
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Function to cleanup processes on exit
cleanup() {
    echo ""
    echo -e "${YELLOW}🛑 Shutting down services...${NC}"
    if [ -n "$BACKEND_PID" ]; then
        kill $BACKEND_PID 2>/dev/null || true
        echo "  ✓ Backend stopped"
    fi
    if [ -n "$FRONTEND_PID" ]; then
        kill $FRONTEND_PID 2>/dev/null || true
        echo "  ✓ Frontend stopped"
    fi
    exit 0
}

# Set trap to cleanup on exit
trap cleanup SIGINT SIGTERM EXIT

echo "📦 Step 1: Checking dependencies..."

# Check if backend binary exists
echo -e "${YELLOW}  Building backend binary...${NC}"
go build -o gslb ./cmd/gslb
echo -e "${GREEN}  ✓ Binary built${NC}"

# Check if frontend dependencies are installed
if [ ! -d "web/node_modules" ]; then
    echo -e "${YELLOW}  Installing frontend dependencies...${NC}"
    cd web && npm install && cd ..
    echo -e "${GREEN}  ✓ Dependencies installed${NC}"
else
    echo -e "${GREEN}  ✓ Frontend dependencies ready${NC}"
fi

echo ""
echo "🔍 Step 2: Checking ports..."

# Kill any processes on port 8090
PORT_8090_PID=$(lsof -t -i:8090 2>/dev/null || netstat -tlnp 2>/dev/null | grep ':8090' | awk '{print $7}' | cut -d'/' -f1 || ss -tlnp 2>/dev/null | grep ':8090' | awk '{print $7}' | cut -d'=' -f2 | cut -d',' -f1)
if [ -n "$PORT_8090_PID" ]; then
    echo -e "${YELLOW}  Killing process on port 8090 (PID: $PORT_8090_PID)${NC}"
    kill -9 $PORT_8090_PID 2>/dev/null || true
    sleep 1
fi
echo -e "${GREEN}  ✓ Port 8090 is free${NC}"

# Kill any processes on port 3000
PORT_3000_PID=$(lsof -t -i:3000 2>/dev/null || netstat -tlnp 2>/dev/null | grep ':3000' | awk '{print $7}' | cut -d'/' -f1 || ss -tlnp 2>/dev/null | grep ':3000' | awk '{print $7}' | cut -d'=' -f2 | cut -d',' -f1)
if [ -n "$PORT_3000_PID" ]; then
    echo -e "${YELLOW}  Killing process on port 3000 (PID: $PORT_3000_PID)${NC}"
    kill -9 $PORT_3000_PID 2>/dev/null || true
    sleep 1
fi
echo -e "${GREEN}  ✓ Port 3000 is free${NC}"

echo ""
echo "🖥️  Step 3: Starting Backend on port 8090..."
./gslb -config config.yaml > /tmp/gslb.log 2>&1 &
BACKEND_PID=$!
sleep 3

# Check if backend is running
if kill -0 $BACKEND_PID 2>/dev/null; then
    echo -e "${GREEN}  ✓ Backend started (PID: $BACKEND_PID)${NC}"
    echo -e "    Logs: tail -f /tmp/gslb.log"
else
    echo -e "${RED}  ✗ Backend failed to start${NC}"
    echo "  Check logs: cat /tmp/gslb.log"
    exit 1
fi

# Quick health check
echo ""
echo "🔍 Step 4: Testing backend API..."
sleep 2
if curl -s http://localhost:8090/api/v1/status > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ Backend API is responding${NC}"
elif curl -s http://localhost:8090/ > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ Backend is running (API routes registered)${NC}"
else
    echo -e "${YELLOW}  ⚠ Backend may still be starting...${NC}"
fi

echo ""
echo "🎨 Step 5: Starting Frontend on port 3000..."
cd web
npm run dev > /tmp/gslb-web.log 2>&1 &
FRONTEND_PID=$!
cd ..
sleep 3

# Check if frontend is running
if kill -0 $FRONTEND_PID 2>/dev/null; then
    echo -e "${GREEN}  ✓ Frontend started (PID: $FRONTEND_PID)${NC}"
    echo -e "    Logs: tail -f /tmp/gslb-web.log"
else
    echo -e "${RED}  ✗ Frontend failed to start${NC}"
    echo "  Check logs: cat /tmp/gslb-web.log"
    exit 1
fi

echo ""
echo "═══════════════════════════════════════════════════"
echo -e "${GREEN}🎉 GSLB Development Environment is Ready!${NC}"
echo "═══════════════════════════════════════════════════"
echo ""
echo "📍 Frontend: http://localhost:3000"
echo "📍 Backend:  http://localhost:8090"
echo "📍 API Docs: http://localhost:8090/api/v1/status"
echo ""
echo "🔧 Proxy Configuration:"
echo "   /api/* -> http://localhost:8090/api/*"
echo ""
echo "📋 Available Commands:"
echo "   tail -f /tmp/gslb.log     # Backend logs"
echo "   tail -f /tmp/gslb-web.log # Frontend logs"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop all services${NC}"
echo ""

# Wait for user to press Ctrl+C
wait
