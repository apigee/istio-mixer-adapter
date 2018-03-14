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
tar -zx -C /opt -f /tmp/gcloud.tar.gz

sudo ln -s /opt/google-cloud-sdk/bin/gcloud /usr/bin/gcloud

gcloud --quiet components update
gcloud auth activate-service-account --key-file=${HOME}/gcloud-service-key.json
gcloud config set project "${GCP_PROJECT}"
