#!/usr/bin/env sh
set -eu

test -f "$1"
cp repair/counter.fixed counter.go
