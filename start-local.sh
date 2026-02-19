#!/bin/bash

echo "🛑 Stopping any existing processes..."
pkill -f p2p-image 2>/dev/null || true
sleep 2

echo ""
echo "🚀 Starting gateway in LOCAL MODE..."
echo "   (Processing images locally, no peers required)"
echo ""

./p2p-image -mode=gateway -http-port=8080 -p2p-port=9000 > logs/gateway.log 2>&1 &
GATEWAY_PID=$!

sleep 3

echo "📊 Checking status..."
RESPONSE=$(curl -s http://localhost:8080/api/status 2>/dev/null)
echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"

echo ""
echo "✅ Gateway started successfully!"
echo ""
echo "🌐 Open http://localhost:8080 in your browser"
echo ""
echo "📝 Note: Running in LOCAL mode (no P2P peers)"
echo "   Images will be processed on the gateway itself"
echo ""
echo "📝 Useful commands:"
echo "  Status: curl http://localhost:8080/api/status"
echo "  Logs: tail -f logs/gateway.log"
echo "  Stop: pkill -f p2p-image"
echo ""
