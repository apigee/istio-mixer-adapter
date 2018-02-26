#!/bin/bash

echo "Installing all the things"

if [[ "${GOPATH}" == "" ]]; then
  echo "GOPATH not set, please set it."
  exit 1
fi

if [[ `command -v protoc` == "" ]]; then
  echo "protoc is not installed, please install."
  exit 1
fi

ADAPTER_DIR="${GOPATH}/src/github.com/apigee/istio-mixer-adapter"

if [ ! -d "${ADAPTER_DIR}" ]; then
  echo "could not find istio-mixer-adapter repo, please put it in:"
  echo "${ADAPTER_DIR}"
  exit 1
fi

if [[ `command -v dep` == "" ]]; then
  if [[ `uname` == "Linux" ]]; then
    echo "dep not installed, installing..."
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
  else
    echo "dep not installed, please install it."
    exit 1
  fi
fi

export ISTIO="${GOPATH}/src/istio.io"
mkdir -p "${ISTIO}"

if [ ! -d "${ISTIO}/istio" ]; then
  echo "istio repo not found, fetching and building..."
  cd "${ISTIO}"
  git clone https://github.com/istio/istio
fi

echo "Checking if istio is built..."
cd "${ISTIO}/istio"
make build || exit 1

echo "All dependencies present, setting up adapter."
cd "${ADAPTER_DIR}"
echo "Running dep ensure..."
dep ensure || exit 1
go generate ./... || exit 1
go build ./... || exit 1
go test ./... || exit 1
