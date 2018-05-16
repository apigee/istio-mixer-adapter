#!/usr/bin/env bash

# This script will build a new draft release on Github.
# It should be used within the context of the process here:
#
#  1. set RELEASE env var
#     (eg. `RELEASE=1.0.0-alpha-2`)
#  2. `git checkout -b $RELEASE`
#  3. Make release updates and commit
#     a. update README.md to appropriate versions and instructions
#     b. update the three proxy zip URLs in apigee-istio/cmd/provision/provision.go
# 	    (anticipate the github URLs based on $RELEASE, see step 10)
#     c. update DEFAULT_ISTIO_VERSION in `bin/local_install.sh`
#  4. `git tag ${RELEASE};git push origin --tags`
#     (CircleCI will automatically build and tag image)
#  5. verify the image
#     (gcr.io/apigee-api-management-istio/istio-mixer:$RELEASE)
#  6. `bin/build_release.sh`
#     (creates a draft release on Github)
#  7. `bin/build_auth_proxies.sh`
#     (creates the following files:)
#	    dist/proxy-istio-auth.zip
#	    dist/proxy-istio-secure.zip
#	    dist/proxy-istio-default.zip
#  8. add the proxy zip files to the release
#     (the URLs from step 3.b should point to these)
#  9. edit Github release:
#     a. add mixer version and docker image URL to release notes
#     b. if this is not a pre-release, uncheck `This is a pre-release` checkbox
# 10. publish release on Github
# 11. test release
# 12. submit PR for $RELEASE branch

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
