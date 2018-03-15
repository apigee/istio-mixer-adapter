#!/bin/bash

echo "Checking environment settings..."

if [[ $GCLOUD_SERVICE_KEY == "" ]]; then
  echo "GCLOUD_SERVICE_KEY not set, please set it."
  exit 1
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
  if [[ "${INSTALL_DOCKER}" == "1" ]]; then
    # Docker not installed, install it
    echo "Installing docker client..."
    VER=17.12.1
    wget -O /tmp/docker-$VER.tgz https://download.docker.com/linux/static/stable/x86_64/docker-$VER-ce.tgz || exit 1
    tar -zx -C /tmp -f /tmp/docker-$VER.tgz
    mv /tmp/docker/* /usr/bin/
  else
    # Don't install it, just complain.
    echo "docker client not installed, please install it."
    exit 1
  fi
fi

if [[ `command -v gcloud` == "" ]]; then
  if [[ "${INSTALL_GCLOUD}" == "1" ]]; then
    echo "Installing gcloud..."
    wget -O /tmp/gcloud.tar.gz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-193.0.0-linux-x86_64.tar.gz || exit 1
    sudo tar -zx -C /opt -f /tmp/gcloud.tar.gz

    # Need to ln so that `sudo gcloud` works
    sudo ln -s /opt/google-cloud-sdk/bin/gcloud /usr/bin/gcloud
  else
    echo "gcloud not installed, please install it."
    exit 1
  fi
fi

echo "Authenticating service account with GCP..."

echo $GCLOUD_SERVICE_KEY | base64 --decode --ignore-garbage > ${HOME}/gcloud-service-key.json

export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/gcloud-service-key.json

gcloud --quiet components update
gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS || exit 1
gcloud config set project "${GCP_PROJECT}" || exit 1

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
