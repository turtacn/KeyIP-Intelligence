#!/bin/bash
# KeyIP-Intelligence CDP Chrome 启动脚本
# 在 macOS 宿主机上执行（不是在 docker-machine 或容器内）
# 用法: bash harness/launch-chrome-cdp.sh

set -e

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
CHROMIUM="/Applications/Chromium.app/Contents/MacOS/Chromium"

# 杀掉旧的 debug 实例
pkill -f "remote-debugging-port" 2>/dev/null || true
sleep 1

if [ -f "$CHROME" ]; then
    CHROME_BIN="$CHROME"
elif [ -f "$CHROMIUM" ]; then
    CHROME_BIN="$CHROMIUM"
else
    echo "ERROR: Chrome/Chromium not found"
    exit 1
fi

echo "🚀 Starting Chrome with CDP on port 9222..."
echo "   Target: http://192.168.99.100/dashboard"

"$CHROME_BIN" \
    --remote-debugging-port=9222 \
    --user-data-dir=/tmp/chrome-debug-profile \
    --no-first-run \
    --no-default-browser-check \
    --disable-extensions \
    --disable-background-networking \
    --disable-sync \
    --disable-translate \
    --disable-features=TranslateUI \
    --window-size=1400,900 \
    "http://192.168.99.100/dashboard" &

sleep 2

# 验证 CDP 端口
if curl -s http://localhost:9222/json | python3 -m json.tool > /dev/null 2>&1; then
    echo "✅ CDP ready at localhost:9222"
    echo ""
    echo "现在可以运行容器内验证:"
    echo "  node harness/cdp-verify.js"
else
    echo "⚠️  CDP may not be ready, try: curl http://localhost:9222/json"
fi
