#!/bin/sh
set -eu
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES
cd "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
plutil -lint ../../../Library/LaunchAgents/com.sjaconette.dev-workspaces.plist
