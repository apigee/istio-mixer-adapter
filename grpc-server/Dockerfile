FROM scratch
ADD ca-certificates.crt /etc/ssl/certs/
ADD apigee-adapter /
CMD ["/apigee-adapter",":5000","--log_output_level=adapters:debug"]
