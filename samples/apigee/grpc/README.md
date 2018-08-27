1. Build binary

	    GOOS=linux go build -a -installsuffix cgo -o apigee-adapter .
	
2. Build docker image

	    docker build -t apigee-adapter -f Dockerfile .

3. Deploy docker image into Kubernetes

	    kubectl apply -f apigee-adapter.yaml

4. Tail adapter logs

        APIGEE_ADAPTER=$(kubectl get po -l app=apigee-adapter -o 'jsonpath={.items[0].metadata.name}')
        ki logs $APIGEE_ADAPTER -f
