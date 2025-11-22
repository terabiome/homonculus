#!/bin/bash

export TAG=${TAG:-latest}
export CONTAINER_EXEC=${CONTAINER_EXEC:-podman}

export DEST_BASE_IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-base-$TAG
export DEST_IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-$TAG

echo "Pushing $DEST_BASE_IMAGE_NAME"
$CONTAINER_EXEC push $DEST_BASE_IMAGE_NAME

echo "Pushing $DEST_IMAGE_NAME"
$CONTAINER_EXEC push $DEST_IMAGE_NAME
