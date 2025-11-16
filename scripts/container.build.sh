#!/bin/bash
export TAG=${TAG:-latest}
export CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
export IMAGE_NAME=homonculus:base-$TAG

/bin/bash ./scripts/container.prebuild.sh

export BASE_IMAGE_NAME=homonculus:base-$TAG
export IMAGE_NAME=homonculus:$TAG

export WORKDIR=/app/homonculus

$CONTAINER_EXEC build \
    --build-arg BASE_IMAGE=$BASE_IMAGE_NAME \
    -f dockerfiles/Dockerfile.build \
    -t $IMAGE_NAME .

