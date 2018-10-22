#!/bin/bash

go generate

GOOS=windows GOARCH=amd64 go build -o tr_xlsxpreviewer_win_amd64.exe
zip tr_xlsxpreviewer_win_amd64.zip tr_xlsxpreviewer_win_amd64.exe
rm tr_xlsxpreviewer_win_amd64.exe

GOOS=darwin GOARCH=amd64 go build -o tr_xlsxpreviewer_macos
tar czf tr_xlsxpreviewer_macos.tar.gz tr_xlsxpreviewer_macos
rm tr_xlsxpreviewer_macos

GOOS=linux GOARCH=amd64 go build -o tr_xlsxpreviewer_linux_amd64
tar czf tr_xlsxpreviewer_linux_amd64.tar.gz tr_xlsxpreviewer_linux_amd64
rm tr_xlsxpreviewer_linux_amd64