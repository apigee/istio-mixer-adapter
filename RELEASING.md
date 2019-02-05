# Building a new draft release on Github

1. set RELEASE env var
    (eg. `RELEASE=1.0.0-alpha-2`)
    
2. create a release branch: `git checkout -b $RELEASE-prep`

3. make release updates
    1. update README.md to appropriate versions and instructions
    2. update DEFAULT_ISTIO_VERSION in `bin/local_install.sh` to match Istio release
    3. update `install/mixer/helm.yaml` to match $RELEASE
    4. update version in `auth-proxy/apiproxy/policies/Send-Version.xml` to match $RELEASE
    5. run `bin/build_proxy_resources.sh`

4. Validate build
    1. update `Gopkg.toml`, ensure appropriate version for `istio.io/istio`
    2. update deps: `dep ensure --update`
    3. remove existing istio from $GOPATH: `rm -rf $GOPATH/src/istio.io/istio`
    4. build mixer: `bin/local_install.sh`

5. Commit and push
    1. verify your changes for git: `git status`
    2. add and commit: `git commit -am "prep ${RELEASE}"`
    3. tag the commit: `git tag ${RELEASE}`
    4. push: `git push --set-upstream origin $RELEASE-prep ${RELEASE}`
 (CircleCI will automatically build and tag docker image)

6. verify the image
    (gcr.io/apigee-api-management-istio/istio-mixer:$RELEASE)

7. `bin/build_release.sh`
    (creates a draft release on Github)

8. edit Github release:
    a. add mixer version and docker image URL to release notes
    b. if this is not a pre-release, uncheck `This is a pre-release` checkbox

9. submit PR for $RELEASE-prep branch

10. merge and final verifications

11. publish release on Github
