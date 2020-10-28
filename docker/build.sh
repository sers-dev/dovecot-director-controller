#!/bin/bash

docker build --tag dovecot-director-controller:latest -f $(dirname "$0")/Dockerfile $(dirname "$0")/..