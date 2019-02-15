#!/bin/bash

# This script will install gcloud on the local machine. Not recommended for use
# on development machines, it is mainly used for containers in CI.

if [[ `command -v gcloud` != "" ]]; then
  echo "gcloud already installed."
  exit 0
fi

echo "Installing gcloud..."
wget -O /tmp/gcloud.tar.gz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-234.0.0-linux-x86_64.tar.gz || exit 1
sudo tar -zx -C /opt -f /tmp/gcloud.tar.gz

# Need to ln so that `sudo gcloud` works
sudo ln -s /opt/google-cloud-sdk/bin/gcloud /usr/bin/gcloud
