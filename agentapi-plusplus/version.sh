#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
    echo "go is required to run this script" && exit 1
elif ! command -v jq >/dev/null 2>&1; then
    echo "jq is required to run this script" && exit 1
fi

version=$(go run main.go --version | awk '{print $3}')

jq --sort-keys --arg version "${version}" '.info.version = $version' openapi.json > openapi.json.tmp && mv openapi.json.tmp openapi.json
jq --sort-keys --arg version "${version}" '.version = $version' chat/package.json > chat/package.json.tmp && mv chat/package.json.tmp chat/package.json

echo -n "${version}"
