#!/bin/bash

export TAG=${TAG:-latest}
export CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
export IMAGE_NAME=localhost/homonculus:$TAG
export DEST_IMAGE_NAME=nhatanhcd2169/terabiome:homonculus-$TAG

echo "Pushing $IMAGE_NAME to $DEST_IMAGE_NAME"

$CONTAINER_EXEC push $IMAGE_NAME $DEST_IMAGE_NAME

export IMAGE_NAME=localhost/homonculus:base-$TAG
export DEST_IMAGE_NAME=nhatanhcd2169/terabiome:homonculus-base-$TAG

echo "Pushing $IMAGE_NAME to $DEST_IMAGE_NAME"

$CONTAINER_EXEC push $IMAGE_NAME $DEST_IMAGE_NAME
