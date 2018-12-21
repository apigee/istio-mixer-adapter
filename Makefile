TAG?=latest

build-osx:
	docker build --build-arg GOOS=darwin --build-arg GOARCH=amd64 -t apigee/apigee-istio:$(TAG) .
	@docker create --name apigee-istio  apigee/apigee-istio:$(TAG) \
	&& docker cp apigee-istio:/usr/bin/apigee-istio apigee-istio/apigee-istio \
	&& docker rm -f apigee-istio
	
build-windows:
	docker build --build-arg GOOS=windows --build-arg GOARCH=amd64 -t apigee/apigee-istio:$(TAG) .
	@docker create --name apigee-istio apigee/apigee-istio:$(TAG) \
	&& docker cp apigee-istio:/usr/bin/apigee-istio apigee-istio/apigee-istio \
	&& docker rm -f apigee-istio