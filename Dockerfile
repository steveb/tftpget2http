ARG GOLANG_BUILDER=golang:1.18
ARG OPERATOR_BASE_IMAGE=gcr.io/distroless/static:nonroot

# Build the tftpget2http binary
FROM $GOLANG_BUILDER AS builder

#Arguments required by OSBS build system
ARG CACHITO_ENV_FILE=/remote-source/cachito.env

ARG REMOTE_SOURCE=.
ARG REMOTE_SOURCE_DIR=/remote-source
ARG REMOTE_SOURCE_SUBDIR=
ARG DEST_ROOT=/dest-root

ARG GO_BUILD_EXTRA_ARGS=

COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR
WORKDIR $REMOTE_SOURCE_DIR/$REMOTE_SOURCE_SUBDIR

RUN mkdir -p ${DEST_ROOT}/usr/local/bin/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN if [ ! -f $CACHITO_ENV_FILE ]; then go mod download ; fi

# Build tftpget2http
RUN if [ -f $CACHITO_ENV_FILE ] ; then source $CACHITO_ENV_FILE ; fi ; CGO_ENABLED=0  GO111MODULE=on go build ${GO_BUILD_EXTRA_ARGS} -a -o ${DEST_ROOT}/tftpget2http main.go

# Use distroless as minimal base image to package the tftpget2http binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM $OPERATOR_BASE_IMAGE

ARG DEST_ROOT=/dest-root
# NONROOT default id https://github.com/GoogleContainerTools/distroless/blob/main/base/base.bzl#L8=
ARG USER_ID=65532

ARG IMAGE_COMPONENT="tftpget2http-container"
ARG IMAGE_NAME="tftpget2http"
ARG IMAGE_VERSION="1.0.0"
ARG IMAGE_SUMMARY="TFTP to HTTP read-only proxy server"
ARG IMAGE_DESC="This image includes a daemon binary which exposes a read-only TFTP server and serves files from a backing HTTP server"
ARG IMAGE_TAGS=""

### DO NOT EDIT LINES BELOW
# Auto generated using CI tools from
# https://github.com/openstack-k8s-operators/openstack-k8s-operators-ci

# Labels required by upstream and osbs build system
LABEL com.redhat.component="${IMAGE_COMPONENT}" \
      name="${IMAGE_NAME}" \
      version="${IMAGE_VERSION}" \
      summary="${IMAGE_SUMMARY}" \
      io.k8s.name="${IMAGE_NAME}" \
      io.k8s.description="${IMAGE_DESC}" \
      io.openshift.tags="${IMAGE_TAGS}"
### DO NOT EDIT LINES ABOVE

ENV USER_UID=$USER_ID

WORKDIR /

USER $USER_ID

# Install tftpget2http binary to WORKDIR
COPY --from=builder ${DEST_ROOT}/tftpget2http .

ENV PATH="/:${PATH}"

CMD ["/tftpget2http"]
