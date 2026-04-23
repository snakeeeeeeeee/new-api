#!/usr/bin/env sh

set -eu

IMAGE="${IMAGE:-ikun0x00/new-api:latest}"
ACTION="${1:-all}"

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

usage() {
  echo "Usage: $0 [build|push|all]"
  echo "Default action: all"
  echo "Override image with: IMAGE=your-registry/your-image:tag $0 all"
}

build_image() {
  echo "==> Building image: $IMAGE"
  docker build -t "$IMAGE" .
}

push_image() {
  echo "==> Pushing image: $IMAGE"
  docker push "$IMAGE"
}

case "$ACTION" in
  build)
    build_image
    ;;
  push)
    push_image
    ;;
  all)
    build_image
    push_image
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
