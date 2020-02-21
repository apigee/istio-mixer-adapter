#!/usr/bin/env bash
golint ./apigee/... ./mixer/... || exit 1
go vet ./apigee-istio/... ./mixer/... ./apigee/... || exit 1
