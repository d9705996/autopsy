#!/usr/bin/env bash
set -euo pipefail

echo "[1/4] formatting check"
unformatted="$(gofmt -l . | tr -d '\r')"
if [[ -n "$unformatted" ]]; then
  echo "Unformatted Go files detected:"
  echo "$unformatted"
  exit 1
fi

echo "[2/4] run tests"
go test ./...

echo "[3/4] accidental artifacts check"
if git status --short | grep -E '(^| )\?\? (autopsy\.db|.*\.db)$' >/dev/null; then
  echo "Database artifact detected in working tree. Remove it before committing."
  git status --short
  exit 1
fi

echo "[4/4] final status"
git status --short

echo "Pre-PR checks passed."
