#!/bin/bash

# This script will install Docker on the local machine. Not recommended for use
# on development machines, it is mainly used for containers in CI.

if [[ `command -v docker` != "" ]]; then
  echo "Docker already installed."
  exit 0
fi

echo "Installing docker client..."
VER=17.12.1
wget -O /tmp/docker-$VER.tgz https://download.docker.com/linux/static/stable/x86_64/docker-$VER-ce.tgz || exit 1
tar -zx -C /tmp -f /tmp/docker-$VER.tgz
mv /tmp/docker/* /usr/bin/
