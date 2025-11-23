#!/bin/bash

TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-$TAG

$CONTAINER_EXEC run \
    -it \
    --name homonculus \
    --network=host \
    --detach \
    --restart unless-stopped \
    -v /var/run/libvirt/libvirt-sock:/var/run/libvirt/libvirt-sock:Z \
    -v /var/lib/libvirt:/var/lib/libvirt:Z \
    -v ~/.ssh:/root/.ssh:ro \
    --privileged \
    --entrypoint homonculus \
    $IMAGE_NAME server
