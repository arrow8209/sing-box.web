#!/bin/sh

set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
npm ci --prefix web
npm run build:web
go test ./...
mkdir -p dist
version=$(git rev-parse --short HEAD)
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$version" -o dist/singbox-web ./cmd/singbox-web

