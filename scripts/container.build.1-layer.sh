#!/bin/bash
export TAG=${TAG:-latest}
export CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
export IMAGE_NAME=homonculus:base-single-layer-$TAG

/bin/bash ./scripts/container.prebuild.sh

export BASE_IMAGE_NAME=homonculus:base-single-layer-$TAG
export IMAGE_NAME=homonculus:single-layer-$TAG

$CONTAINER_EXEC build \
    --build-arg BASE_IMAGE=$BASE_IMAGE_NAME \
    -f dockerfiles/Dockerfile.build.1-layer \
    -t $IMAGE_NAME .

export REMOTE_BASE_IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-base-single-layer-$TAG
export REMOTE_IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-single-layer-$TAG

$CONTAINER_EXEC tag $BASE_IMAGE_NAME $REMOTE_BASE_IMAGE_NAME
$CONTAINER_EXEC tag $IMAGE_NAME $REMOTE_IMAGE_NAME
