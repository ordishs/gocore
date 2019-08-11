#!/bin/sh

cd $(dirname "$0")

SETTINGS_CONTEXT=test go run -race main.go version.go 