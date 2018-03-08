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
gcloud docker -- push "${TARGET_DOCKER_IMAGE}"
