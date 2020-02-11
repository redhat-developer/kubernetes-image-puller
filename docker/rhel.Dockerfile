# Copyright (c) 2018-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# NOTE: using registry.access.redhat.com/rhel8/go-toolset does not work (user is requested to use registry.redhat.io)
# NOTE: using registry.redhat.io/rhel8/go-toolset requires login, which complicates automation
# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/devtools/go-toolset-rhel7
FROM devtools/go-toolset-rhel7:1.12.12-4  as builder
ENV PATH=/opt/rh/go-toolset-1.12/root/usr/bin:$PATH \
    GOPATH=/go/
USER root
WORKDIR /go/src/github.com/redhat-developer/kubernetes-image-puller/
COPY . .

RUN adduser appuser && \
    GOOS=linux go build -o ./bin/kubernetes-image-puller ./cmd/main.go

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM ubi8-minimal:8.1-398
USER root
# CRW-528 copy actual cert
COPY --from=builder /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /etc/pki/ca-trust/extracted/pem/
# CRW-528 copy symlink to the above cert
COPY --from=builder /etc/pki/tls/certs/ca-bundle.crt                  /etc/pki/tls/certs/
COPY --from=builder /etc/passwd /etc/passwd

USER appuser
COPY --from=builder /go/src/github.com/redhat-developer/kubernetes-image-puller/bin/kubernetes-image-puller / 
# TODO need at least these ENV vars to be set when using the image puller; what defaults should we set in the container? 
# export IMAGES=?
# export IMPERSONATE_USERS=?
# export SERVICE_ACCOUNT_ID=?
# export SERVICE_ACCOUNT_SECRET=?
# export OIDC_PROVIDER=?
# export OPENSHIFT_PROXY_URL=?
# export CACHING_INTERVAL_HOURS=?
# export KUBERNETES_SERVICE_HOST=?
# export KUBERNETES_SERVICE_POST=?
CMD ["/kubernetes-image-puller"]

# append Brew metadata here
