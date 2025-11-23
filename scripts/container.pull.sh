#!/bin/bash

export TAG=${TAG:-latest}
export CONTAINER_EXEC=${CONTAINER_EXEC:-podman}

export DEST_BASE_IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-base-$TAG
export DEST_IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-$TAG

echo "Pulling $DEST_BASE_IMAGE_NAME"
$CONTAINER_EXEC pull $DEST_BASE_IMAGE_NAME

echo "Pulling $DEST_IMAGE_NAME"
$CONTAINER_EXEC pull $DEST_IMAGE_NAME
