FROM golang:1.9 as builder
RUN go get -u github.com/Masterminds/glide/...
RUN git clone https://github.com/apid/apid.git /go/src/github.com/apid/apid
WORKDIR /go/src/github.com/apid/apid
RUN glide install --strip-vendor
RUN GOOS=linux go build -a

FROM alpine:latest
RUN apk --no-cache add ca-certificates
# Alpine doesn't have the same lib names as the Linux used above, so link them...
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
WORKDIR /root/
COPY --from=builder /go/src/github.com/apid/apid/apid .
CMD ["./apid"]
