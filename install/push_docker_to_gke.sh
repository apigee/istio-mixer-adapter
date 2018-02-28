#!/bin/bash

echo "Checking environment settings..."

if [[ "${GCP_PROJECT}" == "" ]]; then
  echo "GCP_PROJECT not set, please set it."
  exit 1
fi

if [[ "${DOCKER_IMAGE}" == "" ]]; then
  DOCKER_IMAGE="gcr.io/${GCP_PROJECT}/istio-mixer"
  echo "DOCKER_IMAGE not set, defaulting to ${DOCKER_IMAGE}."
fi

if [[ "${GKE_CLUSTER_NAME}" == "" ]]; then
  GKE_CLUSTER_NAME=apigee-istio-mixer
  echo "GKE_CLUSTER_NAME not set, defaulting to ${GKE_CLUSTER_NAME}."
fi

if [[ "${GKE_DEPLOYMENT_NAME}" == "" ]]; then
  GKE_DEPLOYMENT_NAME=istio-mixer
  echo "GKE_DEPLOYMENT_NAME not set, defaulting to ${GKE_DEPLOYMENT_NAME}."
fi

if [[ "${PORT}" == "" ]]; then
  PORT=8080
  echo "PORT not set, defaulting to ${PORT}."
fi

gcloud config set project "${GCP_PROJECT}"

CLUSTER=$(gcloud container clusters list | grep "${GKE_CLUSTER_NAME}")
if [[ "${CLUSTER}" == "" ]]; then
  echo "GKE cluster ${GKE_CLUSTER_NAME} does not exist, creating..."
  gcloud container clusters create "${GKE_CLUSTER_NAME}"
fi

gcloud container clusters get-credentials "${GKE_CLUSTER_NAME}"
kubectl run "${GKE_DEPLOYMENT_NAME}" --image "${DOCKER_IMAGE}" --port "${PORT}"
kubectl expose deployment "${GKE_DEPLOYMENT_NAME}" --type LoadBalancer
