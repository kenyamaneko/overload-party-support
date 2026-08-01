#!/usr/bin/env bash
set -euo pipefail

pip install \
  "overload-party-doc-tools @ git+https://github.com/kenyamaneko/overload-party-common.git@main#subdirectory=packages/doc-tools"

scripts/generate_types.sh
python3 scripts/generate_schema_doc.py
