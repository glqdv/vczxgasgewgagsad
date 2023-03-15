#!/bin/bash

if !  grep -q  not  <<< $(which go-bindata ) ; then 
    echo "installed "
else 
    echo "not installed "
    go get -u github.com/go-bindata/go-bindata;
    go install github.com/go-bindata/go-bindata/go-bindata;
fi


go-bindata -o=./asset/asset.go -pkg=asset Resources/...