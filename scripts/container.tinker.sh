#!/bin/bash

TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-single-layer-$TAG

$CONTAINER_EXEC run \
    -it \
    --rm \
    --network=host \
    -v /var/run/libvirt/libvirt-sock:/var/run/libvirt/libvirt-sock:Z \
    -v /var/lib/libvirt:/var/lib/libvirt:Z \
    -v ~/.ssh:/root/.ssh:ro \
    -v $(pwd):/app/homonculus \
    --privileged \
    $IMAGE_NAME $@
