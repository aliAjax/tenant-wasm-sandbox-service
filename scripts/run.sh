#!/bin/sh
set -eu
exec go run ./cmd/server -config configs/config.yaml
