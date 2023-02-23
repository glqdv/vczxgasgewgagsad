#!/bin/bash
SERVER=$1
cd ../../ && ./update_res.sh && cd cmds/z-client && sleep 1 ;
echo "Wait 1 s "
if [[ $2 == "armv7" ]] ; then
  GOARCH=arm GOARM=7 GOOS=linux   go build -ldflags="-s -w " -trimpath -o z-client-arm  && scp ./z-client-arm $1:
else
  GOOS=linux   go build -ldflags="-s -w " -trimpath -o z-client-arm  && scp ./z-client-arm $1:
fi
echo "wait to set"
sleep 1
ssh $1 -t  'rm /usr/local/bin/proxy-z ; rm /usr/sbin/redsocks2 ; rm /etc/init.d/proxy-z ; rm /etc/rc.d/S96proxy-z'
ssh $1 -t  ./z-client-arm -http
echo "restart "

