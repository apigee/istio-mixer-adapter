Build binary

    GOOS=linux go build -a -installsuffix cgo -o apigee-adapter .
	
Build docker image

    docker build -t apigee-adapter -f Dockerfile .
	
Deploy docker image into Kubernetes

    kubectl apply -f apigee-adapter.yaml


FYI: This is how to get root certs file for Docker image if needed:

    curl -o ca-certificates.crt https://curl.haxx.se/ca/cacert.pem
