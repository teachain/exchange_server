#!/bin/bash
set -e

BUILD_DIR="build"
SERVICES="accesshttp accessws alertcenter marketprice matchengine readhistory"

mkdir -p "$BUILD_DIR/bin" "$BUILD_DIR/etc"

echo "Building services..."
for svc in $SERVICES; do
    echo "  Building $svc..."
    go build -o "$BUILD_DIR/bin/$svc" "./cmd/$svc"
done

echo "Copying configs..."
for svc in $SERVICES; do
    mkdir -p "$BUILD_DIR/etc/$svc"
    cp "cmd/$svc/config.yaml" "$BUILD_DIR/etc/$svc/"
done

echo "Done. Binaries in $BUILD_DIR/bin/, configs in $BUILD_DIR/etc/"
