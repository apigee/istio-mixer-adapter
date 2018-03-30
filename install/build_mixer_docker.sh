#!/bin/bash

# This script will take the compiled mixer, construct a Docker image for it,
# and upload it to GCR.
#
# Prereqs:
# - run the local_install.sh script to build the mixer.
# - docker is installed.
# - gcloud is installed.
# - GOPATH is set.
#
# Variables:
# - GCLOUD_SERVICE_KEY - auth key for the service account (used in CI to build
#   nightlies)
# - GCP_PROJECT - which GCP_PROJECT to upload the image to.
# - TARGET_DOCKER_IMAGE - the name of the docker image to build.

echo "Checking environment settings..."

if [[ $GCLOUD_SERVICE_KEY == "" ]]; then
  echo "GCLOUD_SERVICE_KEY not set, not using service account."
fi

if [[ $GCP_PROJECT == "" ]]; then
  echo "GCP_PROJECT not set, please set it."
  exit 1
fi

if [[ "${TARGET_DOCKER_IMAGE}" == "" ]]; then
  TARGET_DOCKER_IMAGE="gcr.io/${GCP_PROJECT}/istio-mixer"
  echo "TARGET_DOCKER_IMAGE not set, defaulting to ${TARGET_DOCKER_IMAGE}."
fi

if [[ `command -v docker` == "" ]]; then
  echo "docker client not installed, please install it: ./install/install_docker.sh"
  exit 1
fi

if [[ `command -v gcloud` == "" ]]; then
  echo "gcloud not installed, please install it: ./install/install_gcloud.sh"
  exit 1
fi

gcloud --quiet components update
gcloud config set project "${GCP_PROJECT}" || exit 1

if [[ $GCLOUD_SERVICE_KEY != "" ]]; then
  echo "Authenticating service account with GCP..."
  echo $GCLOUD_SERVICE_KEY | base64 --decode --ignore-garbage > ${HOME}/gcloud-service-key.json

  export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/gcloud-service-key.json
  gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS || exit 1

  echo "Need sudo to install docker-credential-gcr, requesting password..."
  sudo gcloud components install docker-credential-gcr || exit 1

  if [[ `command -v docker-credential-gcr` == "" ]]; then
    # It should be installed, so check if it is in the temp dir.
    if [ -d /opt/google-cloud-sdk/bin ]; then
      export PATH=$PATH:/opt/google-cloud-sdk/bin
    else
      echo "Not able to find docker-credential-gcr, you may need to add the GCP SDK to your PATH."
      exit 1
    fi
  fi

  docker-credential-gcr configure-docker || exit 1

  docker login -u _json_key -p "$(cat ${HOME}/gcloud-service-key.json)" https://gcr.io || exit 1
fi

echo "Building mixer image..."
export ISTIO="${GOPATH}/src/istio.io"

if [ ! -d "${ISTIO}/istio" ]; then
  echo "istio repo not found, please run local_install.sh to set it up."
  exit 1
fi

cd "${ISTIO}/istio"

make docker.mixer || exit 1

IMAGE_ID=$(docker images istio/mixer --format "{{.ID}}" | head -n1)

if [[ "${IMAGE_ID}" == "" ]]; then
  echo "No image found for istio/mixer. Does it exist?"
  exit 1
fi

docker tag "${IMAGE_ID}" "${TARGET_DOCKER_IMAGE}" || exit 1
echo "Pushing to GCR..."
gcloud docker -- push "${TARGET_DOCKER_IMAGE}" || exit 1

if [[ "${MAKE_PUBLIC}" == "1" ]]; then
  gsutil iam ch allUsers:objectViewer "gs://artifacts.${GCP_PROJECT}.appspot.com/"
fi
