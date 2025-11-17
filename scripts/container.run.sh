#!/bin/bash

TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
IMAGE_NAME=localhost/homonculus:$TAG

$CONTAINER_EXEC run \
    -it \
    --rm \
    -v /var/run/libvirt/libvirt-sock:/var/run/libvirt/libvirt-sock:Z \
    -v /var/lib/libvirt:/var/lib/libvirt:Z \
    --privileged \
    --entrypoint homonculus \
    $IMAGE_NAME $@
