Build binary and docker image:

    bin/build_adapter_docker.sh
	
Deploy docker image into Kubernetes

    kubectl apply -f samples/apigee/adapter.yaml

FYI: If needed, root certs file is created via:

    curl -o ca-certificates.crt https://curl.haxx.se/ca/cacert.pem
