#!/usr/bin/env bash

#
# Note: If you build new proxies, you need to publish them and
# point apigee-istio/cmd/provision/provision.go to correct locations.
#

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

DIST_DIR="${ADAPTER_DIR}/dist"
if [ ! -d "${DIST_DIR}" ]; then
  mkdir "${DIST_DIR}"
fi

cd "${DIST_DIR}"
rm -rf "${ADAPTER_DIR}/dist/apiproxy"
cp -R "${ADAPTER_DIR}/auth-proxy/apiproxy" "${ADAPTER_DIR}/dist/apiproxy"

# both vhosts
ZIP=${ADAPTER_DIR}/dist/proxy-istio-auth.zip
echo "building ${ZIP}"
rm -f "${ZIP}"
zip -qr "${ZIP}" apiproxy

# secure only
ZIP=${ADAPTER_DIR}/dist/proxy-istio-secure.zip
echo "building ${ZIP}"
PROXIES_FILE="${DIST_DIR}/apiproxy/proxies/default.xml"
sed -i '' -e 's#<VirtualHost>default</VirtualHost>##g' ${PROXIES_FILE}
rm -f "${ZIP}"
zip -qr "${ZIP}" apiproxy

# default only
ZIP=${ADAPTER_DIR}/dist/proxy-istio-default.zip
echo "building ${ZIP}"
PROXIES_FILE="${DIST_DIR}/apiproxy/proxies/default.xml"
sed -i '' -e 's#<VirtualHost>secure</VirtualHost>#<VirtualHost>default</VirtualHost>#g' ${PROXIES_FILE}
rm -f "${ZIP}"
zip -qr "${ZIP}" apiproxy

echo "done"
