#!/bin/bash

echo "================================"
echo "系統狀態檢查"
echo "================================"

# 檢查服務
if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
    echo "✅ 服務運行中"
    echo "   - API: http://localhost:8080/api"
    echo "   - 前端: http://localhost:8080/web"
else
    echo "❌ 服務未運行"
fi

echo "================================"

# 檢查埠號佔用
echo ""
echo "埠號使用情況:"
lsof -i :8080 2>/dev/null | grep LISTEN && echo "Port 8080 被佔用" || echo "Port 8080 可用"