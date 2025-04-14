#!/bin/sh

set -eu

CGO_ENABLED=0
export CGO_ENABLED

VERSION=$(cat VERSION)

rm -rf dist/v$VERSION
BASEPATH=dist/v$VERSION/virtual1403-agent-v${VERSION}_
mkdir -p ${BASEPATH}{freebsd-amd64,linux-amd64,linux-armv7,windows-amd64,macos-amd64,macos-aarch64}

echo "Building for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-amd64/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for linux/armv6..."
GOOS=linux GOARCH=arm GOARM=6 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-armv6/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for linux/arm64v6..."
GOOS=linux GOARCH=arm64 GOARM=6 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-arm64v6/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for linux/armv7 (e.g. Raspberry Pi)..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-armv7/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for linux/arm64v7 (e.g. Raspberry Pi 64-bit)..."
GOOS=linux GOARCH=arm64 GOARM=7 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-arm64v7/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for freebsd/amd64..."
GOOS=freebsd GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}freebsd-amd64/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}windows-amd64/virtual1403.exe github.com/racingmars/virtual1403/agent

echo "Building for Intel Mac..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}macos-amd64/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for ARM Mac..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}macos-aarch64/virtual1403 github.com/racingmars/virtual1403/agent

for x in freebsd-amd64 linux-amd64 linux-armv7 linux-arm64v7 linux-armv6 linux-arm64v6 windows-amd64 macos-amd64 macos-aarch64; do
    cp agent/config.sample.yaml ${BASEPATH}${x}/config.yaml
    cp agent/README ${BASEPATH}${x}
    cp COPYING ${BASEPATH}${x}
done

# On Windows, make the text files more user-friendly
mv ${BASEPATH}windows-amd64/README ${BASEPATH}windows-amd64/README.txt
mv ${BASEPATH}windows-amd64/COPYING ${BASEPATH}windows-amd64/COPYING.txt
unix2dos ${BASEPATH}windows-amd64/config.yaml ${BASEPATH}windows-amd64/README.txt ${BASEPATH}windows-amd64/COPYING.txt

# ZIP files for Mac and Windows
for x in windows-amd64 macos-amd64 macos-aarch64; do
    cd dist/v$VERSION
    zip -r virtual1403-agent-v${VERSION}_${x}.zip virtual1403-agent-v${VERSION}_${x}
    cd ../..
done

# tarballs for Linux/FreeBSD
for x in freebsd-amd64 linux-amd64 linux-armv7 linux-arm64v7 linux-armv6 linux-arm64v6; do
    cd dist/v$VERSION
    tar czf virtual1403-agent-v${VERSION}_${x}.tgz virtual1403-agent-v${VERSION}_${x}
    cd ../..
done

echo "Done -- artifacts in dist/v$VERSION/"
