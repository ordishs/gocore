#!/bin/sh

go build -o build/darwin/gcli -ldflags="-s -w"
env GOOS=linux GOARCH=amd64 go build -o build/linux/gcli -ldflags="-s -w"

upx --brute build/linux/gcli