#!/usr/bin/env bash
set -e

EDITION="Enterprise"
TAGS=("ee")
SERVE_MODE=false

for arg in "$@"; do
  case "$arg" in
    --ce|-ce)
      EDITION="Community"
      TAGS=()
      ;;
    --serve)
      SERVE_MODE=true
      ;;
  esac
done

if [ "$SERVE_MODE" = false ]; then
  TAGS+=("embed_ui")

  rm -rf ui/dist
  cp -r ../web/dist ui/dist
fi

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")
ENVIRONMENT="${ENV:-unknown}"
BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")
BUILT_BY=$(whoami 2>/dev/null || echo "unknown")
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GO_VERSION=$(go version 2>/dev/null | awk '{print $3}' || echo "unknown")
TARGET_ARCH="$(go env GOOS 2>/dev/null || echo "unknown")"/"$(go env GOARCH 2>/dev/null || echo "unknown")"

LDFLAGS="-X platrium/internal/build.Version=${VERSION} \
-X platrium/internal/build.Edition=${EDITION} \
-X platrium/internal/build.Environment=${ENVIRONMENT} \
-X platrium/internal/build.BuildTime=${BUILD_TIME} \
-X platrium/internal/build.BuiltBy=${BUILT_BY} \
-X platrium/internal/build.CommitSHA=${COMMIT_SHA} \
-X platrium/internal/build.GitBranch=${GIT_BRANCH} \
-X platrium/internal/build.GoVersion=${GO_VERSION} \
-X platrium/internal/build.TargetArch=${TARGET_ARCH}"

TAG_FLAG=""
if [ ${#TAGS[@]} -gt 0 ]; then
  IFS=,
  JOINED_TAGS="${TAGS[*]}"
  unset IFS
  TAG_FLAG="-tags ${JOINED_TAGS}"
fi

if [ "$SERVE_MODE" = true ]; then
  echo "Serving Platrium Core (${EDITION} Edition)..."
  exec go run ${TAG_FLAG} -ldflags "${LDFLAGS}" cmd/engine/main.go
else
  echo "Building Platrium Core (${EDITION} Edition)..."
  echo "  Version:    ${VERSION}"
  echo "  Branch/SHA: ${GIT_BRANCH} @ ${COMMIT_SHA}"
  echo "  Build Time: ${BUILD_TIME}"

  mkdir -p bin
  go build ${TAG_FLAG} -ldflags "${LDFLAGS}" -o bin/engine cmd/engine/main.go
fi