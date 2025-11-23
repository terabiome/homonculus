#!/bin/bash

TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-$TAG
CONTAINER_NAME=homonculus

$CONTAINER_EXEC stop $CONTAINER_NAME

if [[ -n $REMOVE ]]; then
    if [[ -n $FORCE ]]; then
        echo "Detect force remove flag, forcefully exterminating container ..."
        export FORCE='--force'
    else
        echo "Detect remove flag, pruning container ..."
    fi
    $CONTAINER_EXEC rm $FORCE $CONTAINER_NAME
fi
