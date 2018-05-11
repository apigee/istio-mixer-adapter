#!/usr/bin/env bash
if [[ "${GOPATH}" == "" ]]; then
  echo "GOPATH not set, please set it."
  exit 1
fi

if [[ `command -v goreleaser` == "" ]]; then
  echo "goreleaser not installed, installing..."
  go get github.com/goreleaser/goreleaser
fi

ADAPTER_DIR="${GOPATH}/src/github.com/apigee/istio-mixer-adapter"

if [ ! -d "${ADAPTER_DIR}" ]; then
  echo "could not find istio-mixer-adapter repo, please put it in:"
  echo "${ADAPTER_DIR}"
  exit 1
fi

cd "${ADAPTER_DIR}"
goreleaser --rm-dist
