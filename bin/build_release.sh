#!/usr/bin/env bash

# This script will build a new draft release on Github.
# See RELEASING.md for documentation of full release process.

# use DRYRUN=1 to test build

if [[ "${GOPATH}" == "" ]]; then
  echo "GOPATH not set, please set it."
  exit 1
fi

if [[ `command -v goreleaser` == "" ]]; then
  echo "goreleaser not installed, installing..."
  cd "${GOPATH}/bin/"
  wget https://github.com/goreleaser/goreleaser/releases/download/v0.117.1/goreleaser_Linux_x86_64.tar.gz
  tar xfz goreleaser_Linux_x86_64.tar.gz goreleaser
  rm goreleaser_Linux_x86_64.tar.gz
fi

ADAPTER_DIR="${GOPATH}/src/github.com/apigee/istio-mixer-adapter"

if [ ! -d "${ADAPTER_DIR}" ]; then
  echo "could not find istio-mixer-adapter repo, please put it in:"
  echo "${ADAPTER_DIR}"
  exit 1
fi

DRYRUN_ARGS=""
if [[ "${DRYRUN}" == "1" ]]; then
  echo "Dry run, will not label or push to Github"
  DRYRUN_ARGS="--snapshot"
fi


cd "${ADAPTER_DIR}"
goreleaser --rm-dist ${DRYRUN_ARGS}
