#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build"
SERVICES="accesshttp accessws alertcenter marketprice matchengine readhistory"
LOG_DIR="$PROJECT_DIR/logs"

mkdir -p "$LOG_DIR"

echo "Building and starting services..."

for svc in $SERVICES; do
    echo "  Starting $svc..."
    nohup "$BUILD_DIR/bin/$svc" -config "$BUILD_DIR/etc/$svc/config.yaml" > "$LOG_DIR/$svc.log" 2>&1 &
    echo "    PID: $!"
done

echo ""
echo "All services started. Logs in $LOG_DIR/"
echo "Use ./scripts/stop.sh to stop all services."
