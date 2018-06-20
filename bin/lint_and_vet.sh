#!/usr/bin/env bash
golint ./adapter/... || exit 1
go vet ./apigee-istio/... ./adapter/... || exit 1
