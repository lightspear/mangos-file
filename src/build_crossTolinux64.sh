#!/bin/bash
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
export GOEXE=.out
go build -ldflags "-s -w" -o src.out
mkdir -p ../Client
rm -rvf ../Client/mangos-filetransfer.out
upx -o ../Client/mangos-filetransfer.out src.out
rm -rvf src.out