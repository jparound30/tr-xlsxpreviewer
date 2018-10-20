#!/bin/bash

go-assets-builder assets -o assets.go

GOOS=windows GOARCH=amd64 go build -o tr_xlsxpreviewer_win_amd64.exe
GOOS=darwin GOARCH=amd64 go build -o tr_xlsxpreviewer_macos
GOOS=linux GOARCH=amd64 go build -o tr_xlsxpreviewer_linux_amd64
