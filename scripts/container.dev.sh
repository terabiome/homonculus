#!/bin/bash
TAG=${TAG:-latest}
CONTAINER_EXEC=${CONTAINER_EXEC:-podman}

GOPATH=/app/go
WORKDIR=/app/homonculus

$CONTAINER_EXEC build -f prebuild.Dockerfile -t homonculus:$TAG .
$CONTAINER_EXEC run \
    -it \
    --rm \
    -e GOPATH=$GOPATH \
    -v $(pwd):$WORKDIR:Z \
    --workdir $WORKDIR \
    localhost/homonculus:$TAG $@
