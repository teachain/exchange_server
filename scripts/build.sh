#!/bin/bash
set -e

BUILD_DIR="build"
ETC_DIR="etc"
SERVICES="accesshttp accessws alertcenter marketprice matchengine readhistory"

mkdir -p "$BUILD_DIR" "$ETC_DIR"

echo "Building services..."
for svc in $SERVICES; do
    echo "  Building $svc..."
    go build -o "$BUILD_DIR/$svc" "./cmd/$svc"
done

echo "Copying configs..."
for svc in $SERVICES; do
    mkdir -p "$ETC_DIR/$svc"
    cp "cmd/$svc/config.yaml" "$ETC_DIR/$svc/"
done

echo "Done. Binaries in $BUILD_DIR/, configs in $ETC_DIR/"
