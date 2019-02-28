FROM ubuntu:xenial
ADD ca-certificates.crt /etc/ssl/certs/
ADD grpc_health_probe /
ADD apigee-adapter /
CMD ["/apigee-adapter",":5000","--log_output_level=adapters:debug"]
