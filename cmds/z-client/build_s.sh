#!/bin/bash


cd ../../
./update_res.sh
echo "update res"

cd cmds/z-client ;
sleep 1;
go install -trimpath -ldflags="-s -w"

