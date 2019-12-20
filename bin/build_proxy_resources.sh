#!/usr/bin/env bash

#
# If you change the proxies, you must run this and check in the generated proxies.go.
# Remember to update the returned proxy version(s).
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
PROXIES_ZIP_DIR="${DIST_DIR}/proxies"
PROXIES_SOURCE_DIR="${ADAPTER_DIR}/proxies"

LEGACY_AUTH_PROXY_SRC="${PROXIES_SOURCE_DIR}/auth-proxy-legacy"
INTERNAL_PROXY_SRC="${PROXIES_SOURCE_DIR}/internal-proxy"
HYBRID_AUTH_PROXY_SRC="${PROXIES_SOURCE_DIR}/auth-proxy-hybrid"

if [ ! -d "${ADAPTER_DIR}" ]; then
  echo "could not find istio-mixer-adapter repo, please put it in:"
  echo "${ADAPTER_DIR}"
  exit 1
fi

if [ ! -d "${PROXIES_ZIP_DIR}" ]; then
  mkdir -p "${PROXIES_ZIP_DIR}"
fi

# legacy saas auth proxy
ZIP=${PROXIES_ZIP_DIR}/istio-auth-legacy.zip
echo "building ${ZIP}"
rm -f "${ZIP}"
cd "${LEGACY_AUTH_PROXY_SRC}"
zip -qr "${ZIP}" apiproxy

# hybrid auth proxy
ZIP=${PROXIES_ZIP_DIR}/istio-auth-hybrid.zip
echo "building ${ZIP}"
rm -f "${ZIP}"
cd "${HYBRID_AUTH_PROXY_SRC}"
zip -qr "${ZIP}" apiproxy

# internal proxy
ZIP=${PROXIES_ZIP_DIR}/istio-internal.zip
echo "building ${ZIP}"
rm -f "${ZIP}"
cd "${INTERNAL_PROXY_SRC}"
zip -qr "${ZIP}" apiproxy

# create resource
RESOURCE_FILE="${ADAPTER_DIR}/apigee-istio/proxies/proxies.go"
echo "building ${RESOURCE_FILE}"
cd "${DIST_DIR}"
go-bindata -nomemcopy -pkg "proxies" -prefix "proxies" -o "${RESOURCE_FILE}" proxies

echo "done"
