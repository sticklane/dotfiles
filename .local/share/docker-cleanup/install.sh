#!/bin/sh
set -eu
cd /Users/sjaconette/.local/share/docker-cleanup
./check.sh
go build -trimpath -o docker-cleanup .
chmod +x /Users/sjaconette/.local/bin/docker-cleanup
