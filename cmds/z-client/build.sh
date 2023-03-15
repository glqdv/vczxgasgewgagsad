#!/bin/bash

cd ../../ && ./update_res.sh
cd cmds/z-client ;
go install -trimpath -ldflags="-s -w" 
