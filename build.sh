#!/bin/sh

CGO_ENABLED=0
export CGO_ENABLED

mkdir -p dist

echo "Building for linux/amd64..."
GOOS=linux   GOARCH=amd64       go build -trimpath -o dist/virtual1403           github.com/racingmars/virtual1403/agent
echo "Building for linux/armv7 (e.g. Raspberry Pi)..."
GOOS=linux   GOARCH=arm GOARM=7 go build -trimpath -o dist/virtual1403_arm       github.com/racingmars/virtual1403/agent
echo "Building for windows/amd64..."
GOOS=windows GOARCH=amd64       go build -trimpath -o dist/virtual1403.exe       github.com/racingmars/virtual1403/agent
echo "Building for Intel Mac..."
GOOS=darwin  GOARCH=amd64       go build -trimpath -o dist/virtual1403_mac_amd64 github.com/racingmars/virtual1403/agent
echo "Building for ARM Mac..."
GOOS=darwin  GOARCH=arm64       go build -trimpath -o dist/virtual1403_mac_arm64 github.com/racingmars/virtual1403/agent

cp agent/config.sample.yaml dist/config.yaml
cp agent/README.md dist
cp agent/IBMPlexMono-Regular.license dist
cp COPYING dist

echo "Done -- artifacts in dist/"
