#!/bin/sh
sh -c "/docker/reload.sh &"
exec "$@"
