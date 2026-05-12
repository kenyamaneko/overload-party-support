#!/usr/bin/env bash
# generate_types.sh — data/openapi.yaml から packages/api-support (Go) と packages/api-support-npm (TS) の型を再生成する。
#
# REST のみ (本リポは Pub/Sub event を持たないため AsyncAPI codegen は呼ばない)。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$REPO_ROOT/packages/api-support"
oapi-codegen -config openapi-codegen.yaml ../../data/openapi.yaml

cd "$REPO_ROOT"
npx --yes openapi-typescript@7 \
  data/openapi.yaml \
  --output packages/api-support-npm/src/openapi.gen.ts
