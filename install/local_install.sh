#!/bin/bash

echo "Installing all the things"

if [[ "${GOPATH}" == "" ]]; then
  echo "GOPATH not set, please set it."
  exit 1
fi

ADAPTER_DIR="${GOPATH}/src/github.com/apigee/istio-mixer-adapter"

if [ ! -d "${ADAPTER_DIR}" ]; then
  echo "could not find istio-mixer-adapter repo, please put it in:"
  echo "${ADAPTER_DIR}"
  exit 1
fi

if [[ `command -v protoc` == "" ]]; then
  if [[ "${INSTALL_PROTOC}" == "1" ]]; then
    echo "protoc not installed, installing..."
    mkdir /tmp/protoc
    wget -O /tmp/protoc/protoc.zip https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip
    unzip /tmp/protoc/protoc.zip -d /tmp/protoc
    sudo mv -f /tmp/protoc/bin/protoc /usr/bin/
    sudo mv -f /tmp/protoc/include/google /usr/local/include/
    rm -rf /tmp/protoc
  else
    echo "protoc is not installed, install or run with INSTALL_PROTOC=1."
    exit 1
  fi
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

PROTOBUF_DIR="${GOPATH}/src/github.com/gogo/protobuf"

if [ ! -d "${PROTOBUF_DIR}" ]; then
  echo "gogo protobuf not found, fetching..."
  cd "${GOPATH}/src"
  mkdir -p github.com/gogo
  cd github.com/gogo
  git clone https://github.com/gogo/protobuf
fi

export ISTIO="${GOPATH}/src/istio.io"
mkdir -p "${ISTIO}"

if [ ! -d "${ISTIO}/istio" ]; then
  echo "istio repo not found, fetching and building..."
  cd "${ISTIO}"
  git clone https://github.com/istio/istio

  echo "Checking if istio is built..."
  cd "${ISTIO}/istio"
  make depend || exit 1
  make mixs || exit 1
fi

echo "All dependencies present, setting up adapter..."
cd "${ADAPTER_DIR}"
echo "Running dep ensure..."
echo "If this fails it is possible things have changed, try deleting your" \
  "vendor directory and Gopkg.lock and attempting again."
dep ensure || exit 1
go generate ./... || exit 1
go build ./... || exit 1
go test ./... || exit 1

echo "Re-building mixer with Apigee adapter..."

rm -rf "${ADAPTER_DIR}/vendor"
go get github.com/lestrrat/go-jwx
go get github.com/lestrrat/go-pdebug

ln -sf "${GOPATH}/src/github.com/lestrrat" \
  "${ISTIO}/istio/vendor/github.com/lestrrat"
ln -sf "${GOPATH}/src/github.com/apigee" \
  "${ISTIO}/istio/vendor/github.com/apigee"
ln -sf "${GOPATH}/src/github.com/gogo/protobuf/protobuf" \
  "${ISTIO}/istio/vendor/github.com/gogo/protobuf/protobuf"

ADAPTER_FILE="${ISTIO}/istio/mixer/adapter/inventory.yaml"
if [[ `grep "istio-mixer-adapter" "${ADAPTER_FILE}"` == "" ]]; then
  echo "Adding apigee adapter to inventory..."
  echo "
apigee: \"github.com/apigee/istio-mixer-adapter/apigee\"" >> \
    "${ADAPTER_FILE}"
fi

TEMPLATE_FILE="${ISTIO}/istio/mixer/template/inventory.yaml"
if [[ `grep "istio-mixer-adapter" "${TEMPLATE_FILE}"` == "" ]]; then
  echo "Adding apigee adapter template to inventory..."
  echo "
../../../../github.com/apigee/istio-mixer-adapter/template/analytics/template_proto.descriptor_set: \"github.com/apigee/istio-mixer-adapter/template/analytics\"" \
    >> "${TEMPLATE_FILE}"
fi

cd "${ISTIO}/istio"
go generate mixer/adapter/doc.go || exit 1
go generate mixer/template/doc.go || exit 1
make mixs
