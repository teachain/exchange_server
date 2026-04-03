#!/bin/bash
NAME=readhistory
CONFIG=${1:-config.yaml}
PID=$(pgrep -x $NAME || true)
if [ -n "$PID" ]; then
    kill -TERM $PID
    sleep 2
fi
./$NAME -config $CONFIG
