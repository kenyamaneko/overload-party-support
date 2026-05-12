#!/usr/bin/env bash
# 算出済みタグから npm パッケージを build し package.json に version を反映する。
# 入力: TAG (env、prefix `packages/api-support-npm/v` を含む完全タグ名)、PWD は packages/api-support-npm/。
set -euo pipefail

: "${TAG:?TAG env required}"

version="${TAG#packages/api-support-npm/v}"
npm install
npm version "${version}" --no-git-tag-version
npm run build
