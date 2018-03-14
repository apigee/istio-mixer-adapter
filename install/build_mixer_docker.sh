#!/bin/bash

echo "Checking environment settings..."

if [[ "${TARGET_DOCKER_IMAGE}" == "" ]]; then
  if [[ "${GCP_PROJECT}" == "" ]]; then
    echo "TARGET_DOCKER_IMAGE not set, please set it."
    echo "For example: TARGET_DOCKER_IMAGE=gcr.io/robbrit-test/istio-mixer"
    exit 1
  fi

  TARGET_DOCKER_IMAGE="gcr.io/${GCP_PROJECT}/istio-mixer"
  echo "TARGET_DOCKER_IMAGE not set, defaulting to ${TARGET_DOCKER_IMAGE}."
fi

if [[ `command -v docker` == "" ]]; then
  if [[ "${INSTALL_DOCKER}" == "1" ]]; then
    # Docker not installed, install it
    echo "Installing docker client..."
    VER=17.12.1
    wget -O /tmp/docker-$VER.tgz https://download.docker.com/linux/static/stable/x86_64/docker-$VER-ce.tgz
    tar -zx -C /tmp -f /tmp/docker-$VER.tgz
    mv /tmp/docker/* /usr/bin/
  else
    # Don't install it, just complain.
    echo "docker client not installed, please install it."
    exit 1
  fi
fi

export ISTIO="${GOPATH}/src/istio.io"

if [ ! -d "${ISTIO}/istio" ]; then
  echo "istio repo not found, please run local_install.sh to set it up."
  exit 1
fi

cd "${ISTIO}/istio"

make docker.mixer

IMAGE_ID=$(docker images istio/mixer --format "{{.ID}}" | head -n1)

if [[ "${IMAGE_ID}" == "" ]]; then
  echo "No image found for istio/mixer. Does it exist?"
  exit 1
fi

docker tag "${IMAGE_ID}" "${TARGET_DOCKER_IMAGE}"

if [[ "${DOCKER_USERNAME}" != "" && "${DOCKER_PASSWORD}" != "" ]]; then
  echo "Pushing image to Docker Hub..."
  docker login --username "${DOCKER_USERNAME}" --password "${DOCKER_PASSWORD}"
  docker push "${TARGET_DOCKER_IMAGE}"
fi
