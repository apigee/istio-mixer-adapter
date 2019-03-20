# Building a new draft release on Github

1. set RELEASE env var
    (eg. `RELEASE=1.1.0`)
    
2. create a release branch: `git checkout -b $RELEASE-prep`

3. make release updates
    1. update README.md to appropriate versions and instructions
    2. update version in `auth-proxy/apiproxy/policies/Send-Version.xml` to match $RELEASE
    3. run `bin/build_proxy_resources.sh`
    4. update image version in `samples/apigee/grpc/apigee-adapter.yaml` to match $RELEASE

4. for Istio 1.0.x releases
    1. update DEFAULT_ISTIO_VERSION in `bin/local_install.sh`
    2. update `install/mixer/helm.yaml` to match DEFAULT_ISTIO_VERSION
    3. update `Gopkg.toml`, ensure appropriate version for `istio.io/istio`
    4. update deps: `dep ensure --update`
    5. remove existing istio from $GOPATH: `rm -rf $GOPATH/src/istio.io/istio`
    6. build mixer: `bin/local_install.sh`

5. Commit and push
    1. verify your changes for git: `git status`
    2. add and commit: `git commit -am "prep ${RELEASE}"`
    3. tag the commit: `git tag ${RELEASE}`
    4. push: `git push --set-upstream origin $RELEASE-prep ${RELEASE}`
    (CircleCI will automatically build and tag docker image)

6. verify the image
    a. for Istio 1.0.x releases, verify mixer: gcr.io/apigee-api-management-istio/istio-mixer:$RELEASE
    b. for newer releases, verify adapter: gcr.io/apigee-api-management-istio/apigee-adapter:$RELEASE

7. `bin/build_release.sh`
    (creates a draft release on Github)

8. edit Github release:
    a. add mixer version and docker image URL to release notes
    b. if this is not a pre-release, uncheck `This is a pre-release` checkbox

9. submit PR for $RELEASE-prep branch

10. merge and final verifications

11. publish release on Github
