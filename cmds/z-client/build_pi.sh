#!/bin/bash
cd ../../ && ./update_res.sh && cd cmds/z-client && sleep 1 ;
echo "Wait 1 s "
GOOS=linux   go build -ldflags="-s -w " -trimpath -o z-client-arm  && scp ./z-client-arm root@192.168.2.1:
echo "wait to set"
sleep 1
ssh root@192.168.2.1 -t ./clean.sh 
ssh root@192.168.2.1 -t  ./z-client-arm -http
echo "restart "

ssh root@192.168.2.1 -t  reboot
