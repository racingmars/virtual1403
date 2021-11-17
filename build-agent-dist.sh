#!/bin/sh

set -eu

CGO_ENABLED=0
export CGO_ENABLED

VERSION=$(cat VERSION)

rm -rf dist/v$VERSION
BASEPATH=dist/v$VERSION/virtual1403-agent-v${VERSION}_
mkdir -p ${BASEPATH}{linux-amd64,linux-armv7,windows-amd64,macos-amd64,macos-aarch64}

echo "Building for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-amd64/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for linux/armv7 (e.g. Raspberry Pi)..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}linux-armv7/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}windows-amd64/virtual1403.exe github.com/racingmars/virtual1403/agent

echo "Building for Intel Mac..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}macos-amd64/virtual1403 github.com/racingmars/virtual1403/agent

echo "Building for ARM Mac..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$VERSION" -trimpath -o ${BASEPATH}macos-aarch64/virtual1403 github.com/racingmars/virtual1403/agent

for x in linux-amd64 linux-armv7 windows-amd64 macos-amd64 macos-aarch64; do
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

# tarballs for Linux
for x in linux-amd64 linux-armv7; do
    cd dist/v$VERSION
    tar czf virtual1403-agent-v${VERSION}_${x}.tgz virtual1403-agent-v${VERSION}_${x}
    cd ../..
done

echo "Done -- artifacts in dist/v$VERSION/"
