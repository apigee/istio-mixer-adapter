#!/usr/bin/env bash

# This script will build a new draft release on Github.
# It should be used within the context of the process here:
#
#  1. set RELEASE env var
#     (eg. `RELEASE=1.0.0-alpha-2`)
#  2. create a release branch: `git checkout -b $RELEASE`
#  3. make release updates
#     a. update README.md to appropriate versions and instructions as necessary
#     b. update DEFAULT_ISTIO_VERSION in `bin/local_install.sh` as necessary
#     c. replace `samples/istio/istio-demo.yaml`, `istio-demo-auth.yaml`, `helloworld.yaml` from base Istio
#     d. update helloworld.yaml to include Istio sidecar: `istioctl kube-inject -f helloworld.yaml`
#     e. update `samples/istio/istio-demo.yaml`, `samples/istio/istio-demo-auth.yaml` mixer images
#     f. update version in `auth-proxy/apiproxy/policies/Send-Version.xml`
#     g. run `bin/build_proxy_resources.sh`
#     h. commit `git commit -am ${RELEASE}`
#  4. create tag and push: `git tag ${RELEASE};git push origin --tags`
#     (CircleCI will automatically build and tag docker image)
#  5. verify the image
#     (gcr.io/apigee-api-management-istio/istio-mixer:$RELEASE)
#  6. `bin/build_release.sh`
#     (creates a draft release on Github)
#  7. edit Github release:
#     a. add mixer version and docker image URL to release notes
#     b. if this is not a pre-release, uncheck `This is a pre-release` checkbox
#  8. submit PR for $RELEASE branch
#  9. merge and final verifications
# 10. publish release on Github

# use DRYRUN=1 to test build

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

DRYRUN_ARGS=""
if [[ "${DRYRUN}" == "1" ]]; then
  echo "Dry run, will not label or push to Github"
  DRYRUN_ARGS="--skip-publish --skip-validate"
fi


cd "${ADAPTER_DIR}"
goreleaser --rm-dist ${DRYRUN_ARGS}
