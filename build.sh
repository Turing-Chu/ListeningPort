#!/bin/bash

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
flags="-X \"main.buildTime=$(date +'%Y-%m-%d %H:%M:%S')\" -X \"main.commitHash=$(git log --pretty=format:'%h' -n 1)\" -X \"main.branch=$(git branch|awk '{print $2}')\""
go build -o listening_port -ldflags "${flags}" main.go

# just for Turing Zhu's Mac Book Pro
upload listening_port /tmp/


# Reference
# https://goenning.net/2017/01/25/adding-custom-data-go-binaries-compile-time/
# https://github.com/golang/go/issues/12152
