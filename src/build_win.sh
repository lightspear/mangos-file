#!/bin/bash
go build -ldflags "-s -w" -o src.exe ./main.go
# -o ../Client/hycrecv1.exe
mkdir -p ../Client
rm -rvf ../Client/mangos-filetransfer.exe
upx -o ../Client/mangos-filetransfer.exe src.exe
rm -rvf src.exe
