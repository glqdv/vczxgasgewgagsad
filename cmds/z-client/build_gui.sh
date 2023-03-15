#!/bin/bash

cd ../../ && ./update_res.sh
cd ./cmds/z-client && sleep 1 ;
echo "Go Build"

go build -ldflags="-s -w" -trimpath
sleep 1
cp package.json.raw package.json
echo "build electron"
npm config set registry=https://registry.npm.taobao.org/
npm config set ELECTRON_MIRROR=https://cdn.npm.taobao.org/dist/electron/
npm config set ELECTRON_BUILDER_BINARIES_MIRROR=http://npm.taobao.org/mirrors/electron-builder-binaries/
npm install --save-dev @electron-forge/cli
npx electron-forge import

echo "compile to package"
npm run make