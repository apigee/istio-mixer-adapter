#!/bin/bash

echo "Checking environment settings..."

if [[ "${APIGEE_ORG}" == "" ]]; then
  echo "APIGEE_ORG not set, please set it."
  exit 1
fi

if [[ "${APIGEE_ENV}" == "" ]]; then
  echo "APIGEE_ENV not set, please set it."
  exit 1
fi

if [[ "${APIGEE_KEY}" == "" ]]; then
  echo "APIGEE_KEY not set, please set it."
  exit 1
fi

if [[ "${APIGEE_SECRET}" == "" ]]; then
  echo "APIGEE_SECRET not set, please set it."
  exit 1
fi

if [[ "${GOPATH}" == "" ]]; then
  echo "GOPATH not set, please set it."
  exit 1
fi

export ISTIO="${GOPATH}/src/istio.io"

if [ ! -d "${ISTIO}/istio" ]; then
  echo "istio repo not found, please run local_install.sh to set it up."
  exit 1
fi

GOARCH=amd64  # this is hard-coded by Istio so hopefully they don't change it

if [[ `uname` == "Linux" ]]; then
  GOOS=linux
elif [[ `uname` == "Darwin" ]]; then
  GOOS=darwin
else
  echo "Unknown OS $(uname)"
  exit 1
fi
  

echo "Writing config files..."
CONFIG_FILE="${ISTIO}/istio/mixer/testdata/config/apigee.yaml"

cat "${GOPATH}/src/github.com/apigee/istio-mixer-adapter/testdata/operatorconfig/config.yaml" \
  | sed "s/theganyo1-eval/${APIGEE_ORG}/" \
  | sed "s/test/${APIGEE_ENV}/" \
  > "${CONFIG_FILE}" 

ATTR_FILE="${ISTIO}/istio/mixer/testdata/config/attributes.yaml"

if [[ `grep 'request.auth.claims' "${ATTR_FILE}"` == "" ]]; then
  sed -i \
    -e "/request.auth.principal/ i \      request.auth.claims:" \
    -e "/request.auth.principal/ i \        value_type: STRING_MAP" \
    "${ATTR_FILE}"
fi

MIXS="${GOPATH}/out/${GOOS}_${GOARCH}/release/mixs"

"${MIXS}" server --alsologtostderr \
  "--configStoreURL=fs://${ISTIO}/istio/mixer/testdata/config"
