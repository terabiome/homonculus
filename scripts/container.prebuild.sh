#!/bin/bash
# TAG=${TAG:-latest}
# CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
# IMAGE_NAME=homonculus:base-$TAG

$CONTAINER_EXEC build -f dockerfiles/Dockerfile.prebuild -t $IMAGE_NAME .
