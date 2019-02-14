#!/bin/bash
#
# Copyright (c) 2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0

set -u
set -e
set -x

LOCAL_IMAGE_NAME='kubernetes-image-puller'
REGISTRY='quay.io'
ORGANIZATION='openshiftio'
RHEL_IMAGE_NAME='rhel-kubernetes-image-puller'
CENTOS_IMAGE_NAME='kubernetes-image-puller'

# Simplify tagging and pushing
function tag_and_push() {
  local tag
  tag=$1
  docker tag ${LOCAL_IMAGE_NAME} $tag
  docker push $tag
}

# Cleanup on exit
function cleanup() {
  make clean
  if [ -f "./jenkins-env" ]; then
    rm ~/.jenkins-env
  fi
}
trap cleanup EXIT

# Source build variables
if [ -e "jenkins-env" ]; then
  cat jenkins-env \
    | grep -E "(DEVSHIFT_TAG_LEN|QUAY_USERNAME|QUAY_PASSWORD|GIT_COMMIT)=" \
    | sed 's/^/export /g' \
    > ~/.jenkins-env
  source ~/.jenkins-env
fi

# Update machine, get required deps in place
yum -y update
yum -y install docker golang
systemctl start docker

# Login to quay.io
docker login -u ${QUAY_USERNAME} -p ${QUAY_PASSWORD} ${REGISTRY}

# Build main executable and docker image, push to quay.io
make build
TAG=$(echo $GIT_COMMIT | cut -c1-${DEVSHIFT_TAG_LEN})
if [ "$TARGET" = "rhel" ]; then
  docker build -t ${LOCAL_IMAGE_NAME} -f ./docker/Dockerfile.rhel .
  tag_and_push ${REGISTRY}/${ORGANIZATION}/${RHEL_IMAGE_NAME}:${TAG}
  tag_and_push ${REGISTRY}/${ORGANIZATION}/${RHEL_IMAGE_NAME}:latest
else
  docker build -t ${LOCAL_IMAGE_NAME} -f ./docker/Dockerfile.centos .
  tag_and_push ${REGISTRY}/${ORGANIZATION}/${CENTOS_IMAGE_NAME}:${TAG}
  tag_and_push ${REGISTRY}/${ORGANIZATION}/${CENTOS_IMAGE_NAME}:latest
fi
