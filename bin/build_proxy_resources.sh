#!/usr/bin/env bash

#
# If you change the proxies, you must run this and check in the generated proxies.go.
#

if [[ "${GOPATH}" == "" ]]; then
  echo "GOPATH not set, please set it."
  exit 1
fi

if [[ `command -v go-bindata` == "" ]]; then
  echo "go-bindata not installed, installing..."
  go get -u github.com/go-bindata/go-bindata/...
fi

ADAPTER_DIR="${GOPATH}/src/github.com/apigee/istio-mixer-adapter"
DIST_DIR="${ADAPTER_DIR}/dist"
PROXIES_DIR="${DIST_DIR}/proxies"

AUTH_PROXY_SRC="${ADAPTER_DIR}/auth-proxy/apiproxy"
INTERNAL_PROXY_SRC="${ADAPTER_DIR}/internal-proxy/apiproxy"

if [ ! -d "${ADAPTER_DIR}" ]; then
  echo "could not find istio-mixer-adapter repo, please put it in:"
  echo "${ADAPTER_DIR}"
  exit 1
fi

if [ ! -d "${PROXIES_DIR}" ]; then
  mkdir -p "${PROXIES_DIR}"
fi

PROXY_TEMP_DIR="${DIST_DIR}/apiproxy"
cd "${DIST_DIR}"
rm -rf "${PROXY_TEMP_DIR}"
cp -R "${AUTH_PROXY_SRC}" "${PROXY_TEMP_DIR}"

PROXIES_FILE="${PROXY_TEMP_DIR}/proxies/default.xml"

# auth proxy
ZIP=${PROXIES_DIR}/istio-auth.zip
echo "building ${ZIP}"
rm -f "${ZIP}"
zip -qr "${ZIP}" apiproxy

# internal proxy
ZIP=${PROXIES_DIR}/istio-internal.zip
echo "building ${ZIP}"
rm -rf "${PROXY_TEMP_DIR}"
cp -R "${INTERNAL_PROXY_SRC}" "${PROXY_TEMP_DIR}"
rm -f "${ZIP}"
zip -qr "${ZIP}" apiproxy

# create resource
RESOURCE_FILE="${ADAPTER_DIR}/apigee-istio/proxies/proxies.go"
echo "building ${RESOURCE_FILE}"
go-bindata -nomemcopy -pkg "proxies" -prefix "proxies" -o "${RESOURCE_FILE}" proxies

echo "done"
