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

if [[ `command -v gcloud` == "" ]]; then
  if [[ "${INSTALL_GCLOUD}" == "1" ]]; then
    if [[ $GCLOUD_SERVICE_KEY == "" ]]; then
      echo "GCLOUD_SERVICE_KEY not set, please set it."
      exit 1
    fi

    if [[ $GCP_PROJECT == "" ]]; then
      echo "GCP_PROJECT not set, please set it."
      exit 1
    fi

    echo $GCLOUD_SERVICE_KEY | base64 --decode --ignore-garbage > ${HOME}/gcloud-service-key.json

    echo "Installing gcloud..."
    wget -O /tmp/gcloud.tar.gz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-193.0.0-linux-x86_64.tar.gz
    sudo tar -zx -C /opt -f /tmp/gcloud.tar.gz

    export PATH=$PATH:/opt/google-cloud-sdk/bin
    # Need to ln so that `sudo gcloud` works
    sudo ln -s /opt/google-cloud-sdk/bin/gcloud /usr/bin/gcloud

    export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/gcloud-service-key.json

    gcloud --quiet components update
    gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS
    gcloud config set project "${GCP_PROJECT}"

    sudo gcloud components install docker-credential-gcr
    docker-credential-gcr configure-docker
  else
    echo "gcloud not installed, please install it."
    exit 1
  fi
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
gcloud docker -- push "${TARGET_DOCKER_IMAGE}"
