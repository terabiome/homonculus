#!/bin/bash

TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}
SSH_DIRECTORY=${SSH_DIRECTORY:-~/.ssh}
IMAGE_NAME=docker.io/nhatanhcd2169/terabiome:homonculus-$TAG

CONTAINER_EXEC_OPTS=(\
    "-it" \
    "--name homonculus" \
    "--network=host" \
    "--detach" \
    "--restart always" \
    "-v /var/run/libvirt/libvirt-sock:/var/run/libvirt/libvirt-sock:Z" \
    "-v /var/lib/libvirt:/var/lib/libvirt:Z" \
    "-v $SSH_DIRECTORY:/root/.ssh:ro" \
    "--privileged" \
    "--entrypoint homonculus"
)

# mount YAML config to /app/homonculus directory in container
# if explicitly defined and there exists the file
if [[ -n ${HOMONCULUS_YAML_CONFIG} ]]; then
    echo "Homonculus YAML config file path is defined: $HOMONCULUS_YAML_CONFIG"
    if [[ -f ${HOMONCULUS_YAML_CONFIG} ]]; then
        echo "Found homonculus YAML config file at $HOMONCULUS_YAML_CONFIG, mounting to container.."
        CONTAINER_EXEC_OPTS+=("-v ${HOMONCULUS_YAML_CONFIG}:/app/homonculus/homonculus.yaml")
    else
        echo "Could not find homonculus YAML config file at $HOMONCULUS_YAML_CONFIG, aborting ..."
        exit 1
    fi
else
    echo "Homonculus YAML config file path undefined, using default one in container"
fi

$CONTAINER_EXEC run ${CONTAINER_EXEC_OPTS[@]} $IMAGE_NAME server
