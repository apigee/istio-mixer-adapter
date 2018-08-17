1. Build binary

	    GOOS=linux go build -a -installsuffix cgo -o apigee-adapter .
	
2. Build docker image

	    docker build -t apigee-adapter -f Dockerfile .

3. Deploy docker image into Kubernetes

	    kubectl apply -f apigee-adapter.yaml

        AA=$(kubectl get po -l app=apigee-adapter -o 'jsonpath={.items[0].metadata.name}')
        ki port-forward $AA 5000:5000
        curl 0:5000 -v
        ki logs $AA
