#!/bin/sh

cd $(dirname "$0")

SETTINGS_CONTEXT=test go run main.go version.go 