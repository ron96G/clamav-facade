#!/bin/sh

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o client main.go