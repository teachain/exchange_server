#!/bin/bash

SERVICES="accesshttp accessws alertcenter marketprice matchengine readhistory"

echo "Stopping services..."

for svc in $SERVICES; do
    pid=$(pgrep -f "$svc" | head -1)
    if [ -n "$pid" ]; then
        echo "  Stopping $svc (PID: $pid)..."
        kill "$pid" 2>/dev/null || true
    else
        echo "  $svc not running"
    fi
done

echo "All services stopped."
