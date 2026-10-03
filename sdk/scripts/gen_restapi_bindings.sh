#!/bin/bash
set -e

PROJECT_ROOT="sdk"

if [ "$MODE" = "ee" ]; then
  OPENAPI_FILE="api/rest/_generated/openapi_ee.yaml"
else
  OPENAPI_FILE="api/rest/_generated/openapi.yaml"
fi

echo "Generating Rust SDK bindings using $OPENAPI_FILE ($MODE mode)..."

rm -rf ${PROJECT_ROOT}/_generated/src ${PROJECT_ROOT}/_generated/Cargo.toml

docker run --rm -v "$(pwd)":/local openapitools/openapi-generator-cli generate \
  -i /local/${OPENAPI_FILE} \
  -g rust \
  -o /local/${PROJECT_ROOT}/_generated \
  --package-name platrium-restapi \
  --additional-properties=supportAsync=true,preferPassthroughParameters=true,reqwestDefaultFeatures=reqwest/rustls \
  --global-property apiDocs=false,modelDocs=false,apiTests=false,modelTests=false

rm -rf ${PROJECT_ROOT}/_generated/.openapi-generator \
       ${PROJECT_ROOT}/_generated/git_push.sh \
       ${PROJECT_ROOT}/_generated/README.md \
       ${PROJECT_ROOT}/_generated/.openapi-generator-ignore \
       ${PROJECT_ROOT}/_generated/.travis.yml
