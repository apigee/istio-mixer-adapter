#!/usr/bin/env bash
(echo lint apigee; cd apigee; golint ./...)
(echo lint mixer; cd mixer; golint .)
(echo vet apigee; cd apigee; go vet ./...)
(echo vet mixer; cd mixer; go vet .)
(echo vet apigee-istio; cd apigee-istio; go vet ./...)
