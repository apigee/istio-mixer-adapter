FROM scratch
ADD ca-certificates.crt /etc/ssl/certs/
ADD grpc_health_probe /
ADD apigee-adapter /
ENTRYPOINT ["/apigee-adapter"]
CMD ["--address=:5000", "--log_output_level=adapters:info"]
