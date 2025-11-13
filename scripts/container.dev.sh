#!/bin/bash
TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}

$CONTAINER_EXEC build -f prebuild.Dockerfile -t homonculus:$TAG .
$CONTAINER_EXEC run \
    -it \
    --rm \
    -v $(pwd):/app/:Z \
    localhost/homonculus:$TAG $@
