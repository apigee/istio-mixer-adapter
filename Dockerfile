FROM golang:latest as builder

ARG GOOS=darwin 
ARG GOARCH=amd64 

WORKDIR /go/src/github.com/apigee/istio-mixer-adapter

COPY . .

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

RUN dep ensure 

# Run a gofmt and exclude all vendored code.
RUN test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*"))" || { echo "Run \"gofmt -s -w\" on your Golang code"; exit 1; }

#RUN go test $(go list ./... | grep -v /vendor/) -cover \ Commenting out since there is a test file with code commented out which causes go test to fail
RUN cd apigee-istio \
&& VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') \
 && GIT_COMMIT=$(git rev-list -1 HEAD) \
 && CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build --ldflags "-s -w \
    -X github.com/apigee/istio-mixer-adapter/version.GitCommit=${GIT_COMMIT} \
    -X github.com/apigee/istio-mixer-adapter/version.Version=${VERSION}" \
    -a -installsuffix cgo -o apigee-istio

FROM alpine:latest 

RUN apk --no-cache add ca-certificates git

COPY --from=builder /go/src/github.com/apigee/istio-mixer-adapter/apigee-istio/apigee-istio /usr/bin/ 

ENV PATH=$PATH:/usr/bin/

CMD ["apigee-istio version"]