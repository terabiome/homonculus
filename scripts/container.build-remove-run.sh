#!/bin/bash
sh scripts/container.build.sh
REMOVE=true FORCE=true sh scripts/container.stop.server.sh
sh scripts/container.run.server.sh
