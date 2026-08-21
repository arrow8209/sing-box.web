#!/bin/sh
set -eu

npm --prefix web run build
mkdir -p dist
go build -o dist/singbox-web ./cmd/singbox-web
