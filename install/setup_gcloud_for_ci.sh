#!/bin/bash

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

export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/gcloud-service-key.json

gcloud --quiet components update
gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS
gcloud config set project "${GCP_PROJECT}"

sudo gcloud components install docker-credential-gcr
docker-credential-gcr configure-docker
